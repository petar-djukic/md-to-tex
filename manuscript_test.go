package mdtotex_test

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	mdtotex "github.com/petar-djukic/md-to-tex"
)

// update rewrites the committed expectations instead of comparing against
// them. Run it when a mapping changes on purpose, and read the diff: it is the
// change to the emitted LaTeX, stated in full.
var update = flag.Bool("update", false, "rewrite the expected LaTeX in testdata")

// manuscriptKeys are the citation keys the fixture's corpus holds. The library
// never reads a reference corpus, so the caller states them.
var manuscriptKeys = []string{"du-2023", "coronado-2022-ztn-survey"}

// TestTheManuscriptConverts converts the fixture manuscript end to end and
// compares every fragment and the container against the committed
// expectations.
//
// The SRD examples cover each construct on its own; this covers them together,
// in the shape a paper actually takes: a title page, chapters carrying
// headings, prose, citations, raw LaTeX, a figure, a wide figure, and a table,
// and a container inputting the roster.
func TestTheManuscriptConverts(t *testing.T) {
	roster := manuscriptRoster(t)
	options := mdtotex.Options{CitationKeys: manuscriptKeys}

	for _, chapter := range roster {
		t.Run(chapter, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("testdata", "manuscript", chapter))
			if err != nil {
				t.Fatal(err)
			}

			var fragment []byte
			if chapter == "00-front-matter.md" {
				result, err := mdtotex.RenderFrontMatter(source, chapter, options)
				if err != nil {
					t.Fatalf("RenderFrontMatter() error: %v", err)
				}
				fragment = result.LaTeX
			} else {
				result, err := mdtotex.Convert(source, chapter, options)
				if err != nil {
					t.Fatalf("Convert() error: %v", err)
				}
				fragment = result.LaTeX
			}

			compare(t, strings.TrimSuffix(chapter, ".md")+".tex", fragment)
		})
	}

	t.Run("main.tex", func(t *testing.T) {
		document, err := mdtotex.GenerateContainer(roster, mdtotex.ContainerOptions{})
		if err != nil {
			t.Fatalf("GenerateContainer() error: %v", err)
		}
		compare(t, "main.tex", document)
	})
}

// TestTheManuscriptReportsItsLabels covers the whole-manuscript view of the
// label report: every chapter's identifiers, and no collision across them.
func TestTheManuscriptReportsItsLabels(t *testing.T) {
	options := mdtotex.Options{CitationKeys: manuscriptKeys}

	var results []mdtotex.Result
	for _, chapter := range manuscriptRoster(t) {
		if chapter == "00-front-matter.md" {
			continue
		}
		source, err := os.ReadFile(filepath.Join("testdata", "manuscript", chapter))
		if err != nil {
			t.Fatal(err)
		}
		result, err := mdtotex.Convert(source, chapter, options)
		if err != nil {
			t.Fatalf("Convert(%s) error: %v", chapter, err)
		}
		results = append(results, result)
	}

	if collisions := mdtotex.Collisions(results...); len(collisions) != 0 {
		t.Errorf("the manuscript carries colliding identifiers: %+v", collisions)
	}

	stated := map[string]bool{}
	for _, result := range results {
		for _, label := range result.Labels {
			stated[label.Identifier] = true
		}
	}
	for _, want := range []string{"sec:intro", "sec:floats", "what-the-mapping-fixes"} {
		if !stated[want] {
			t.Errorf("the manuscript did not report the label %q; it reported %v", want, keys(stated))
		}
	}
}

// TestAnUnknownKeyStopsTheManuscript covers the caller-facing failure: a
// citation the corpus does not hold fails the chapter that carries it, naming
// the key, rather than reaching the LaTeX as a question mark in the PDF.
func TestAnUnknownKeyStopsTheManuscript(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "manuscript", "01-introduction.md"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = mdtotex.Convert(source, "01-introduction.md",
		mdtotex.Options{CitationKeys: []string{"du-2023"}})
	if err == nil {
		t.Fatal("Convert() accepted a key the corpus does not hold")
	}
	if !strings.Contains(err.Error(), "coronado-2022-ztn-survey") {
		t.Errorf("error = %q, want it to name the key", err)
	}
}

func manuscriptRoster(t *testing.T) []string {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join("testdata", "manuscript", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("the fixture manuscript holds no chapters")
	}
	roster := make([]string, len(entries))
	for i, entry := range entries {
		roster[i] = filepath.Base(entry)
	}
	sort.Strings(roster)
	return roster
}

// compare holds emitted LaTeX to its committed expectation, or rewrites it
// under -update.
func compare(t *testing.T, name string, emitted []byte) {
	t.Helper()
	path := filepath.Join("testdata", "expected", name)

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, emitted, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v\nrun go test -run TestTheManuscriptConverts -update to write it", err)
	}
	if string(emitted) != string(want) {
		t.Errorf("%s differs from its expectation\n got:\n%s\nwant:\n%s", name, emitted, want)
	}
}

func keys(set map[string]bool) []string {
	list := make([]string, 0, len(set))
	for key := range set {
		list = append(list, key)
	}
	sort.Strings(list)
	return list
}
