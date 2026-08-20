package latex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOneReplacementTable covers srd003-escaping R3.2 and AC5: exactly one
// implementation of the replacement table exists, so the front-matter path and
// the node renderers cannot drift apart.
//
// The marker is the textasciicircum replacement, which appears nowhere else in
// a library that escapes in one place. Tests are exempt: they assert against
// the replacements and would otherwise count as implementations of them.
func TestOneReplacementTable(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	var carriers []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), `textasciicircum`) {
			relative, _ := filepath.Rel(root, path)
			carriers = append(carriers, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(carriers) != 1 || carriers[0] != "internal/latex/escape.go" {
		t.Errorf("replacement table found in %v, want only internal/latex/escape.go", carriers)
	}
}
