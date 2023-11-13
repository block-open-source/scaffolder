package scaffolder

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type scaffoldOptions struct {
	funcs template.FuncMap
}

type Option func(*scaffoldOptions)

// Functions defines functions to use in scaffolding templates.
func Functions(funcs template.FuncMap) Option {
	return func(o *scaffoldOptions) {
		o.funcs = funcs
	}
}

// Scaffold evaluates the scaffolding files at the given source using ctx, then
// copies them into destination.
//
// Both path names and file contents are evaluated.
//
// If a file name ends with `.tmpl`, the `.tmpl` suffix is removed.
//
// Scaffold is inspired by [cookiecutter].
//
// [cookiecutter]: https://github.com/cookiecutter/cookiecutter
func Scaffold(source, destination string, ctx any, options ...Option) error {
	opts := scaffoldOptions{}
	for _, option := range options {
		option(&opts)
	}

	return walkDir(source, func(srcPath string, d fs.DirEntry) error {
		path, err := filepath.Rel(source, srcPath)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if strings.HasSuffix(path, ".tmpl") {
			newPath := strings.TrimSuffix(path, ".tmpl")
			if err = os.Rename(path, newPath); err != nil {
				return fmt.Errorf("failed to rename file: %w", err)
			}
			path = newPath
		}

		// Evaluate the last component of path name templates.
		dir := filepath.Dir(path)
		origName := filepath.Base(path)
		newName, err := evaluate(origName, ctx, opts.funcs)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		dstPath := filepath.Join(destination, dir, newName)

		err = os.Remove(dstPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("%s: %w", dstPath, err)
		}

		dstDir := filepath.Dir(dstPath)
		if err := os.MkdirAll(dstDir, 0700); err != nil {
			return fmt.Errorf("%s: %w", dstPath, err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return fmt.Errorf("%s: %w", srcPath, err)
			}

			target, err = evaluate(target, ctx, opts.funcs)
			if err != nil {
				return fmt.Errorf("%s: %w", srcPath, err)
			}

			// Ensure symlink is relative.
			if filepath.IsAbs(target) {
				rel, err := filepath.Rel(filepath.Dir(dstPath), target)
				if err != nil {
					return err
				}
				target = rel
			}

			return os.Symlink(target, dstPath)
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s: unsupported file type %s", srcPath, info.Mode())
		}

		// Evaluate file content.
		template, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("%s: %w", srcPath, err)
		}
		content, err := evaluate(string(template), ctx, opts.funcs)
		if err != nil {
			return fmt.Errorf("%s: %w", srcPath, err)
		}
		err = os.WriteFile(dstPath, []byte(content), info.Mode())
		if err != nil {
			return fmt.Errorf("%s: %w", dstPath, err)
		}
		return nil
	})
}

// Walk dir executing fn after each entry.
func walkDir(dir string, fn func(path string, d fs.DirEntry) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			err = walkDir(filepath.Join(dir, entry.Name()), fn)
			if err != nil {
				return err
			}
		}
		err = fn(filepath.Join(dir, entry.Name()), entry)
		if err != nil {
			return err
		}
	}
	return nil
}

func evaluate(tmpl string, ctx any, funcs template.FuncMap) (string, error) {
	t, err := template.New("scaffolding").Funcs(funcs).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}
	newName := &strings.Builder{}
	err = t.Execute(newName, ctx)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return newName.String(), nil
}
