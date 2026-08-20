package specs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// layout is a documentation layer written into a temporary directory: one
// architecture file, the SRDs it points at, and the roadmap that schedules
// them. Cases mutate it to produce the state under test.
type layout struct {
	architecture string
	roadMap      string
	srds         map[string]string
	tests        map[string]string
}

const validArchitecture = `id: architecture-test
title: Test Architecture
components:
  - name: Escaper
    srd: docs/srd/srd-1-escaping.yaml
interfaces:
  - name: Conversion
    srd: docs/srd/srd-1-escaping.yaml
`

const validRoadMap = `releases:
  - id: rel00.1
    units:
      - srd-1-escaping
`

const validSRD = `id: srd-1-escaping
title: Escaper
requirements:
  R1:
    title: The escaped characters
    items:
      - R1.1: The escaper must replace the ten LaTeX special characters.
      - R1.2: The escaper must pass Unicode through untouched.
acceptance_criteria:
  - id: AC1
    criterion: A string of specials escapes and nothing else changes.
    traces: [R1.1, R1.2]
`

func write(t *testing.T, l layout) string {
	t.Helper()
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(filepath.Join(docs, "srd"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(docs, "VISION.yaml"), "id: vision-test\ntitle: Test Vision\n")
	writeFile(t, filepath.Join(docs, "ARCHITECTURE.yaml"), l.architecture)
	writeFile(t, filepath.Join(docs, "road-map.yaml"), l.roadMap)
	for name, body := range l.srds {
		writeFile(t, filepath.Join(docs, "srd", name), body)
	}
	for name, body := range l.tests {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, path, body)
	}
	return root
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func valid() layout {
	return layout{
		architecture: validArchitecture,
		roadMap:      validRoadMap,
		srds:         map[string]string{"srd-1-escaping.yaml": validSRD},
	}
}

func TestCheckReportsLayerProblems(t *testing.T) {
	cases := []struct {
		name  string
		build func() layout
		want  string
	}{
		{
			name:  "clean layer",
			build: valid,
			want:  "",
		},
		{
			name: "dangling pointer",
			build: func() layout {
				l := valid()
				l.architecture = strings.Replace(l.architecture,
					"srd: docs/srd/srd-1-escaping.yaml\ninterfaces",
					"srd: docs/srd/srd-2-absent.yaml\ninterfaces", 1)
				return l
			},
			want: "srd-2-absent.yaml is named by Escaper but does not exist",
		},
		{
			name: "orphan specification",
			build: func() layout {
				l := valid()
				l.srds["srd-9-unclaimed.yaml"] = strings.Replace(validSRD,
					"id: srd-1-escaping", "id: srd-9-unclaimed", 1)
				return l
			},
			want: "no component or interface in ARCHITECTURE.yaml names this file",
		},
		{
			name: "duplicate mapping key",
			build: func() layout {
				l := valid()
				l.srds["srd-1-escaping.yaml"] = validSRD + "title: Escaper again\n"
				return l
			},
			want: `mapping key "title" already defined`,
		},
		{
			name: "duplicate key in the architecture",
			build: func() layout {
				l := valid()
				l.architecture += "title: Test Architecture again\n"
				return l
			},
			want: `mapping key "title" already defined`,
		},
		{
			name: "component with no pointer",
			build: func() layout {
				l := valid()
				l.architecture = strings.Replace(l.architecture,
					"  - name: Escaper\n    srd: docs/srd/srd-1-escaping.yaml\n",
					"  - name: Escaper\n    srd: docs/srd/srd-1-escaping.yaml\n  - name: Tables\n", 1)
				return l
			},
			want: `component "Tables" names no srd`,
		},
		{
			name: "id disagrees with the file name",
			build: func() layout {
				l := valid()
				l.srds["srd-1-escaping.yaml"] = strings.Replace(validSRD,
					"id: srd-1-escaping", "id: srd-1-escapes", 1)
				return l
			},
			want: `id "srd-1-escapes" does not match the file name`,
		},
		{
			name: "criterion traces to a requirement that does not exist",
			build: func() layout {
				l := valid()
				l.srds["srd-1-escaping.yaml"] = strings.Replace(validSRD,
					"traces: [R1.1, R1.2]", "traces: [R1.1, R4.7]", 1)
				return l
			},
			want: "criterion AC1 traces to R4.7, which is not a requirement here",
		},
		{
			name: "test names several requirements of one specification",
			build: func() layout {
				l := valid()
				l.tests = map[string]string{
					"convert_test.go": "package convert\n\n// Covers srd-1-escaping R1.1, R1.2, and R9.9.\nfunc TestNothing() {}\n",
				}
				return l
			},
			want: "names srd-1-escaping R9.9, which is not a requirement in the docs layer",
		},
		{
			name: "requirement ids wrapped across comment lines still count",
			build: func() layout {
				l := valid()
				l.tests = map[string]string{
					"convert_test.go": "package convert\n\n// Covers srd-1-escaping R1.1, R1.2, and\n// R9.9 across two lines.\nfunc TestNothing() {}\n",
				}
				return l
			},
			want: "names srd-1-escaping R9.9, which is not a requirement in the docs layer",
		},
		{
			name: "test names a requirement that does not exist",
			build: func() layout {
				l := valid()
				l.tests = map[string]string{
					"convert_test.go": "package convert\n\n// srd-1-escaping R8.3 is not a requirement.\nfunc TestNothing() {}\n",
				}
				return l
			},
			want: "names srd-1-escaping R8.3, which is not a requirement in the docs layer",
		},
		{
			name: "release schedules a specification nobody wrote",
			build: func() layout {
				l := valid()
				l.roadMap = strings.Replace(l.roadMap,
					"- srd-1-escaping", "- srd-1-escaping\n      - srd-2-absent", 1)
				return l
			},
			want: "release rel00.1 names srd-2-absent, which is not an SRD on disk",
		},
		{
			name: "specification no release schedules",
			build: func() layout {
				l := valid()
				l.roadMap = "releases:\n  - id: rel00.1\n    units: []\n"
				return l
			},
			want: "srd-1-escaping is assigned to no release",
		},
		{
			name: "specification scheduled twice",
			build: func() layout {
				l := valid()
				l.roadMap += "  - id: rel00.2\n    units:\n      - srd-1-escaping\n"
				return l
			},
			want: "srd-1-escaping is assigned to rel00.1 and rel00.2; exactly one release owns an SRD",
		},
		{
			name: "duplicate key in the roadmap",
			build: func() layout {
				l := valid()
				l.roadMap += "releases: []\n"
				return l
			},
			want: `mapping key "releases" already defined`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			report, err := Check(write(t, testCase.build()))
			if err != nil {
				t.Fatalf("Check() error: %v", err)
			}
			problems := report.Err()
			if testCase.want == "" {
				if problems != nil {
					t.Fatalf("clean layer reported:\n%v", problems)
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

// Coverage is a report rather than a gate: a requirement no test names is
// counted, and one a test does name is credited.
func TestCoverageCountsWithoutFailing(t *testing.T) {
	l := valid()
	l.tests = map[string]string{
		"convert_test.go": "package convert\n\n// Covers srd-1-escaping R1.1.\nfunc TestEscape() {}\n",
	}
	report, err := Check(write(t, l))
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if problems := report.Err(); problems != nil {
		t.Fatalf("coverage should not fail the run:\n%v", problems)
	}
	if report.Requirements != 2 {
		t.Errorf("Requirements = %d, want 2", report.Requirements)
	}
	if report.Covered != 1 {
		t.Errorf("Covered = %d, want 1", report.Covered)
	}
	if len(report.Uncovered) != 1 || report.Uncovered[0] != "srd-1-escaping R1.2" {
		t.Errorf("Uncovered = %v, want [srd-1-escaping R1.2]", report.Uncovered)
	}
	if !strings.Contains(report.Summary(), "1 of 2 requirements") {
		t.Errorf("summary does not carry the coverage line:\n%s", report.Summary())
	}
}

// TestDocsLayer is the check running against this repository, which is what
// mage audit dispatches to and what CI runs.
func TestDocsLayer(t *testing.T) {
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
	t.Log("\n" + report.Summary())
}
