package specs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoSourceFileIsIgnored covers the failure that produced this test: an
// unanchored artifact pattern in .gitignore claimed a source file, git add
// skipped it without a word, and the commit compiled only in the working tree
// it was made from (GH-41).
//
// Every Go file, YAML document, and markdown file in the tree must be one git
// would track. The check runs against the repository rather than a fixture,
// because the rule it guards is a property of this repository's .gitignore.
func TestNoSourceFileIsIgnored(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is absent; this checks a git ignore rule")
	}

	var sources []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "bin", "build":
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(entry.Name()) {
		case ".go", ".yaml", ".md", ".tex":
			sources = append(sources, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) < 20 {
		t.Fatalf("found %d source files; the walk is not reaching the tree", len(sources))
	}

	// check-ignore exits 0 and prints the paths it would ignore, and exits 1
	// when it would ignore none. Anything it prints is a source file a commit
	// would silently leave behind.
	command := exec.Command("git", append([]string{"check-ignore", "--"}, sources...)...)
	command.Dir = root
	output, _ := command.Output()

	if ignored := strings.TrimSpace(string(output)); ignored != "" {
		t.Errorf("the ignore rules would drop source files from a commit:\n%s", ignored)
	}
}
