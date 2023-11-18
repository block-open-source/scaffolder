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
	funcs   template.FuncMap
	exclude []string
}

type Option func(*scaffoldOptions)

// Functions defines functions to use in scaffolding templates.
func Functions(funcs template.FuncMap) Option {
	return func(o *scaffoldOptions) {
		o.funcs = funcs
	}
}

// Exclude the given relative path prefixes from scaffolding.
func Exclude(paths ...string) Option {
	return func(so *scaffoldOptions) {
		so.exclude = append(so.exclude, paths...)
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

	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(destination, 0700); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	deferredSymlinks := map[string]string{}

	err := walkDir(source, func(srcPath string, d fs.DirEntry) error {
		path, err := filepath.Rel(source, srcPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		for _, exclude := range opts.exclude {
			if strings.HasPrefix(path, exclude) {
				return nil
			}
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info: %w", err)
		}

		dstPath, err := evaluate(path, ctx, opts.funcs)
		if err != nil {
			return fmt.Errorf("failed to evaluate path name: %w", err)
		}
		dstPath = filepath.Join(destination, dstPath)
		dstPath = strings.TrimSuffix(dstPath, ".tmpl")

		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(srcPath)
			if err != nil {
				return fmt.Errorf("failed to read symlink: %w", err)
			}

			target, err = evaluate(target, ctx, opts.funcs)
			if err != nil {
				return fmt.Errorf("failed to evaluate symlink target: %w", err)
			}

			// Ensure symlink is relative.
			if filepath.IsAbs(target) {
				rel, err := filepath.Rel(filepath.Dir(dstPath), target)
				if err != nil {
					return fmt.Errorf("failed to make symlink relative: %w", err)
				}
				target = rel
			}

			deferredSymlinks[dstPath] = target

		case info.Mode().IsDir():
			if err := os.MkdirAll(dstPath, 0700); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

		case info.Mode().IsRegular():
			// Evaluate file content.
			template, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			content, err := evaluate(string(template), ctx, opts.funcs)
			if err != nil {
				return fmt.Errorf("%s: failed to evaluate template: %w", srcPath, err)
			}
			err = os.WriteFile(dstPath, []byte(content), info.Mode())
			if err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

		default:
			return fmt.Errorf("%s: unsupported file type %s", srcPath, info.Mode())
		}
		return nil
	})
	if err != nil {
		return err
	}

	for dstPath := range deferredSymlinks {
		if err := applySymlinks(deferredSymlinks, dstPath); err != nil {
			return fmt.Errorf("failed to apply symlink: %w", err)
		}
	}
	return nil
}

// Recursively apply symlinks.
func applySymlinks(symlinks map[string]string, path string) error {
	target, ok := symlinks[path]
	if !ok {
		return nil
	}
	targetPath := filepath.Clean(filepath.Join(filepath.Dir(path), target))
	if err := applySymlinks(symlinks, targetPath); err != nil {
		return fmt.Errorf("failed to apply symlink: %w", err)
	}
	delete(symlinks, path)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove symlink target: %w", err)
	}
	return os.Symlink(target, path)
}

// Depth-first walk of dir executing fn after each entry.
func walkDir(dir string, fn func(path string, d fs.DirEntry) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			err = fn(filepath.Join(dir, entry.Name()), entry)
			if err != nil {
				return err
			}
			err = walkDir(filepath.Join(dir, entry.Name()), fn)
			if err != nil {
				return err
			}
		} else {
			err = fn(filepath.Join(dir, entry.Name()), entry)
			if err != nil {
				return err
			}
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
