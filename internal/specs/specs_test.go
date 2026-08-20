package specs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const roadMapHeader = "releases:\n"

// layout writes a repository whose corpus holds the named SRDs, whose
// architecture points at each of them, and whose road map assigns units as
// given. Cases override the architecture where that edge is what they test.
func layout(t *testing.T, srds []string, releases map[string][]string, order []string) string {
	t.Helper()
	root := t.TempDir()
	corpus := filepath.Join(root, "docs", "specs", "software-requirements")
	if err := os.MkdirAll(corpus, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range srds {
		write(t, filepath.Join(corpus, name+".yaml"), "id: "+name+"\ntitle: A Document\n")
	}
	write(t, filepath.Join(root, "docs", "ARCHITECTURE.yaml"), architectureFor(srds))

	body := roadMapHeader
	for _, id := range order {
		body += "  - id: " + id + "\n    version: \"" + strings.TrimPrefix(id, "rel") + "\"\n    units:\n"
		for _, unit := range releases[id] {
			body += "      - " + unit + "\n"
		}
	}
	write(t, filepath.Join(root, "docs", "road-map.yaml"), body)
	return root
}

// architectureFor names every SRD from a component, which is the clean state
// of the edge.
func architectureFor(srds []string) string {
	body := "id: architecture-test\ntitle: Test Architecture\ncomponents:\n"
	for i, name := range srds {
		body += fmt.Sprintf("  - name: Component %d\n    srd: docs/specs/software-requirements/%s.yaml\n", i+1, name)
	}
	return body
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCheckWalksTheReleaseEdge covers both directions of the edge the shared
// corpus format has no place for.
func TestCheckWalksTheReleaseEdge(t *testing.T) {
	cases := []struct {
		name     string
		srds     []string
		releases map[string][]string
		order    []string
		want     string
	}{
		{
			name:     "every document owned by exactly one release",
			srds:     []string{"srd001-alpha", "srd002-beta"},
			releases: map[string][]string{"rel00.1": {"srd001-alpha"}, "rel00.2": {"srd002-beta"}},
			order:    []string{"rel00.1", "rel00.2"},
		},
		{
			name:     "a unit naming no document on disk",
			srds:     []string{"srd001-alpha"},
			releases: map[string][]string{"rel00.1": {"srd001-alpha", "srd009-absent"}},
			order:    []string{"rel00.1"},
			want:     "release rel00.1 names srd009-absent, which is not an SRD on disk",
		},
		{
			name:     "a document no release claims",
			srds:     []string{"srd001-alpha", "srd002-beta"},
			releases: map[string][]string{"rel00.1": {"srd001-alpha"}},
			order:    []string{"rel00.1"},
			want:     "srd002-beta is assigned to no release",
		},
		{
			name:     "a document two releases claim",
			srds:     []string{"srd001-alpha"},
			releases: map[string][]string{"rel00.1": {"srd001-alpha"}, "rel00.2": {"srd001-alpha"}},
			order:    []string{"rel00.1", "rel00.2"},
			want:     "srd001-alpha is assigned to rel00.1 and rel00.2",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			report, err := Check(layout(t, testCase.srds, testCase.releases, testCase.order))
			if err != nil {
				t.Fatalf("Check() error: %v", err)
			}
			problems := report.Err()
			if testCase.want == "" {
				if problems != nil {
					t.Fatalf("a clean road map reported:\n%v", problems)
				}
				return
			}
			if problems == nil {
				t.Fatalf("no finding reported; wanted one naming %q", testCase.want)
			}
			if !strings.Contains(problems.Error(), testCase.want) {
				t.Errorf("findings do not name %q:\n%v", testCase.want, problems)
			}
		})
	}
}

// TestCheckWalksTheArchitectureEdge covers the edge the specification-critic
// does not read: ARCHITECTURE.yaml is this repository's own document, and a
// critic run passes with a pointer resolving to nothing.
func TestCheckWalksTheArchitectureEdge(t *testing.T) {
	cases := []struct {
		name         string
		architecture string
		want         string
	}{
		{
			name: "a pointer naming no file",
			architecture: "id: architecture-test\ntitle: Test Architecture\ncomponents:\n" +
				"  - name: Tables\n    srd: docs/specs/software-requirements/srd909-absent.yaml\n",
			want: `component "Tables" names docs/specs/software-requirements/srd909-absent.yaml, which does not exist`,
		},
		{
			name:         "a component naming no requirement document",
			architecture: "id: architecture-test\ntitle: Test Architecture\ncomponents:\n  - name: Tables\n",
			want:         `component "Tables" names no srd`,
		},
		{
			name: "a document no component claims",
			architecture: "id: architecture-test\ntitle: Test Architecture\ncomponents:\n" +
				"  - name: Alpha\n    srd: docs/specs/software-requirements/srd001-alpha.yaml\n",
			want: "no component or interface in ARCHITECTURE.yaml names this file",
		},
		{
			name: "a duplicate mapping key",
			architecture: "id: architecture-test\ntitle: Test Architecture\ntitle: Again\ncomponents:\n" +
				"  - name: Alpha\n    srd: docs/specs/software-requirements/srd001-alpha.yaml\n",
			want: `mapping key "title" already defined`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := layout(t,
				[]string{"srd001-alpha", "srd002-beta"},
				map[string][]string{"rel00.1": {"srd001-alpha", "srd002-beta"}},
				[]string{"rel00.1"})
			write(t, filepath.Join(root, "docs", "ARCHITECTURE.yaml"), testCase.architecture)

			report, err := Check(root)
			if err != nil {
				t.Fatalf("Check() error: %v", err)
			}
			problems := report.Err()
			if problems == nil {
				t.Fatalf("no finding reported; wanted one naming %q", testCase.want)
			}
			if !strings.Contains(problems.Error(), testCase.want) {
				t.Errorf("findings do not name %q:\n%v", testCase.want, problems)
			}
		})
	}
}

// TestCheckCountsWhatItRead covers the summary a human reads.
func TestCheckCountsWhatItRead(t *testing.T) {
	root := layout(t,
		[]string{"srd001-alpha", "srd002-beta"},
		map[string][]string{"rel00.1": {"srd001-alpha", "srd002-beta"}},
		[]string{"rel00.1"})

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if report.SRDFiles != 2 || report.Releases != 1 || report.Pointers != 2 {
		t.Errorf("SRDFiles = %d, Releases = %d, Pointers = %d, want 2, 1, and 2",
			report.SRDFiles, report.Releases, report.Pointers)
	}
	if !strings.Contains(report.Summary(), "1 releases over 2 requirement documents") {
		t.Errorf("Summary() = %q", report.Summary())
	}
}

// TestCheckReportsAnUnreadableRoadMap covers the path where the file is absent
// or malformed: a finding, not a panic.
func TestCheckReportsAnUnreadableRoadMap(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if report.Err() == nil {
		t.Error("a missing road map should be reported")
	}

	write(t, filepath.Join(root, "docs", "road-map.yaml"), "releases: [oops\n")
	report, err = Check(root)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if report.Err() == nil {
		t.Error("a malformed road map should be reported")
	}
}

// TestCheckReadsTheRealRoadMap runs the check against this repository, which
// is what mage audit dispatches to.
func TestCheckReadsTheRealRoadMap(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	report, err := Check(root)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if problems := report.Err(); problems != nil {
		t.Fatal(problems)
	}
	t.Log(report.Summary())
}
