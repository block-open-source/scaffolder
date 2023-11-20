package scaffolder

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestScaffolder(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "new")
	err := Scaffold("testdata/template", tmpDir, map[string]any{
		"Name":    "test",
		"Include": true,
	}, Exclude("excluded"))
	assert.NoError(t, err)
	type file struct {
		name    string
		mode    os.FileMode
		content string
	}
	expect := []file{
		{"include", 0o600, "included"},
		{"included-dir/included", 0o600, "included"},
		{"intermediate", 0o700 | os.ModeSymlink, "Hello, test!\n"},
		{"regular-test", 0o600, "Hello, test!\n"},
		{"symlink-test", 0o700 | os.ModeSymlink, "Hello, test!\n"},
	}
	actual := []file{}
	err = walkDir(tmpDir, func(path string, d os.DirEntry) error {
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(tmpDir, path)
		if err != nil {
			return err
		}
		var content []byte
		if !d.IsDir() {
			content, err = os.ReadFile(path)
			if err != nil {
				return err
			}
			actual = append(actual, file{name: rel, mode: info.Mode() & (os.ModeSymlink | 0o700), content: string(content)})
		}

		return nil
	})
	assert.NoError(t, err)
	sort.Slice(actual, func(i, j int) bool { return actual[i].name < actual[j].name })
	assert.Equal(t, expect, actual)
}
