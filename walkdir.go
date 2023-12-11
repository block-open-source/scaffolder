package scaffolder

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrSkip can be returned by WalkDir callbacks to skip a file or directory.
var ErrSkip = errors.New("skip directory")

// WalkDir performs a depth-first walk of dir, executing fn before each file or
// directory.
//
// If fn returns ErrSkip, the directory will be skipped.
func WalkDir(dir string, fn func(path string, d fs.DirEntry) error) error {
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if err = fn(dir, fs.FileInfoToDirEntry(dirInfo)); err != nil {
		if errors.Is(err, ErrSkip) {
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
			err = WalkDir(filepath.Join(dir, entry.Name()), fn)
			if err != nil && !errors.Is(err, ErrSkip) {
				return err
			}
		} else {
			err = fn(filepath.Join(dir, entry.Name()), entry)
			if err != nil && !errors.Is(err, ErrSkip) {
				return err
			}
		}
	}
	return nil
}
