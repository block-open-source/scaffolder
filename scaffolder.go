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

// Scaffold evaluates the scaffolding files at the given destination against
// ctx.
//
// Both paths and file contents are evaluated.
//
// If a file name ends with `.tmpl`, the `.tmpl` suffix is removed.
//
// The functions `snake`, `screamingSnake`, `camel`, `lowerCamel`, `kebab`,
// `screamingKebab`, `upper`, `lower`, `title`, and `typename`, are available by
// default.
//
// Scaffold is inspired by [cookiecutter].
//
// [cookiecutter]: https://github.com/cookiecutter/cookiecutter
func Scaffold(destination string, ctx any, options ...Option) error {
	opts := scaffoldOptions{}
	for _, option := range options {
		option(&opts)
	}
	return walkDir(destination, func(path string, d fs.DirEntry) error {
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
		base := filepath.Base(path)
		newName, err := evaluate(base, ctx, opts.funcs)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		// Rename if necessary.
		if newName != base {
			newName = filepath.Join(dir, newName)
			err = os.Rename(path, newName)
			if err != nil {
				return fmt.Errorf("failed to rename file: %w", err)
			}
			path = newName
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		// Evaluate file content.
		template, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		content, err := evaluate(string(template), ctx, opts.funcs)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		err = os.WriteFile(path, []byte(content), info.Mode())
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
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
