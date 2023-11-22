package scaffolder

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

type scaffoldOptions struct {
	after   func(path string) error
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

// Exclude the given regex paths from scaffolding.
//
// Matching occurs before template evaluation and .tmpl suffix removal.
func Exclude(paths ...string) Option {
	return func(so *scaffoldOptions) {
		so.exclude = append(so.exclude, paths...)
	}
}

// After configures Scaffolder to call "after" for each file or directory after
// it is created.
func After(after func(path string) error) Option {
	return func(so *scaffoldOptions) { so.after = after }
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
	opts := scaffoldOptions{
		after: func(path string) error { return nil },
	}
	for _, option := range options {
		option(&opts)
	}

	deferredSymlinks := map[string]string{}

	err := walkDir(source, func(srcPath string, d fs.DirEntry) error {
		path, err := filepath.Rel(source, srcPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		for _, exclude := range opts.exclude {
			if matched, err := regexp.MatchString(exclude, path); err != nil {
				return fmt.Errorf("invalid exclude pattern %q: %w", exclude, err)
			} else if matched {
				return nil
			}
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info: %w", err)
		}

		if lastComponent, err := evaluate(filepath.Base(path), ctx, opts.funcs); err != nil {
			return fmt.Errorf("failed to evaluate path name: %w", err)
		} else if lastComponent == "" {
			return errSkip
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
			if err := opts.after(dstPath); err != nil {
				return fmt.Errorf("after directory: %w", err)
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
			if err := opts.after(dstPath); err != nil {
				return fmt.Errorf("after file: %w", err)
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

// errSkip is returned by walkDir to skip a file or directory.
var errSkip = errors.New("skip directory")

// Depth-first walk of dir executing fn after each entry.
func walkDir(dir string, fn func(path string, d fs.DirEntry) error) error {
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if err = fn(dir, fs.FileInfoToDirEntry(dirInfo)); err != nil {
		if errors.Is(err, errSkip) {
			return nil
		}
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			err = walkDir(filepath.Join(dir, entry.Name()), fn)
			if err != nil && !errors.Is(err, errSkip) {
				return err
			}
		} else {
			err = fn(filepath.Join(dir, entry.Name()), entry)
			if err != nil && !errors.Is(err, errSkip) {
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
