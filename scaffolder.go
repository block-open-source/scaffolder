// Package scaffolder is a general purpose file-system based scaffolding tool
// inspired by cookiecutter.
package scaffolder

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

type scaffoldOptions struct {
	Config
	plugins []Extension
}

// Extension's allow the scaffolder to be extended.
type Extension interface {
	Extend(mutableConfig *Config) error
	AfterEach(path string) error
}

// ExtensionFunc is a convenience type for creating an Extension.Extend from a function.
type ExtensionFunc func(mutableConfig *Config) error

func (f ExtensionFunc) Extend(mutableConfig *Config) error { return f(mutableConfig) }
func (f ExtensionFunc) AfterEach(path string) error        { return nil }

// AfterExtensionFunc is a convenience type for creating an Extension.AfterEach from a function.
type AfterExtensionFunc func(path string) error

func (f AfterExtensionFunc) Extend(mutableConfig *Config) error { return nil }
func (f AfterExtensionFunc) AfterEach(path string) error        { return f(path) }

// Option is a function that modifies the behaviour of the scaffolder.
type Option func(*scaffoldOptions)

// FuncMap is a map of functions to use in scaffolding templates.
//
// The key is the function name and the value is a function taking a single
// argument and returning either `string` or `(string, error)`.
type FuncMap = template.FuncMap

// Config for the scaffolding.
type Config struct {
	Context any
	Funcs   FuncMap
	Exclude []string

	source string
	target string
}

func (c *Config) Source() string { return c.source }
func (c *Config) Target() string { return c.target }

// Functions adds functions to use in scaffolding templates.
func Functions(funcs FuncMap) Option {
	return func(o *scaffoldOptions) {
		for k, v := range funcs {
			o.Funcs[k] = v
		}
	}
}

// Extend adds an Extension to the scaffolder.
//
// An extension can be used to add functions to the template context, to
// modify the template context, and so on.
func Extend(plugin Extension) Option {
	return func(o *scaffoldOptions) {
		o.plugins = append(o.plugins, plugin)
	}
}

// Exclude the given regex paths from scaffolding.
//
// Matching occurs before template evaluation and .tmpl suffix removal.
func Exclude(paths ...string) Option {
	return func(so *scaffoldOptions) {
		so.Exclude = append(so.Exclude, paths...)
	}
}

// AfterEach configures Scaffolder to call "after" for each file or directory
// created.
//
// Useful for setting file permissions, etc.
//
// Each AfterEach function is called in order.
func AfterEach(after func(path string) error) Option {
	return func(so *scaffoldOptions) {
		so.plugins = append(so.plugins, AfterExtensionFunc(after))
	}
}

// Scaffold evaluates the scaffolding files at the given source using ctx, while
// copying them into destination.
func Scaffold(source, destination string, ctx any, options ...Option) error {
	opts := scaffoldOptions{
		Config: Config{
			source:  source,
			target:  destination,
			Context: ctx,
			Funcs:   FuncMap{},
		},
	}
	for _, option := range options {
		option(&opts)
	}

	deferredSymlinks := map[string]string{}

	for _, plugin := range opts.plugins {
		if err := plugin.Extend(&opts.Config); err != nil {
			return fmt.Errorf("failed to extend scaffolder: %w", err)
		}
	}

	err := WalkDir(source, func(srcPath string, d fs.DirEntry) error {
		path, err := filepath.Rel(source, srcPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		for _, exclude := range opts.Exclude {
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

		if lastComponent, err := evaluate(filepath.Base(path), ctx, opts.Funcs); err != nil {
			return fmt.Errorf("failed to evaluate path name: %w", err)
		} else if lastComponent == "" {
			return ErrSkip
		}

		dstPath, err := evaluate(path, ctx, opts.Funcs)
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

			target, err = evaluate(target, ctx, opts.Funcs)
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
			for _, plugin := range opts.plugins {
				if err := plugin.AfterEach(dstPath); err != nil {
					return fmt.Errorf("failed to run after: %w", err)
				}
			}

		case info.Mode().IsRegular():
			// Evaluate file content.
			template, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			content, err := evaluate(string(template), ctx, opts.Funcs)
			if err != nil {
				return fmt.Errorf("%s: failed to evaluate template: %w", srcPath, err)
			}
			err = os.WriteFile(dstPath, []byte(content), info.Mode())
			if err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			for _, plugin := range opts.plugins {
				if err := plugin.AfterEach(dstPath); err != nil {
					return fmt.Errorf("failed to run after: %w", err)
				}
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
