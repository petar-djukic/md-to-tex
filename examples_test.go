package mdtotex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	mdtotex "github.com/petar-djukic/md-to-tex"
)

// example is one markdown and LaTeX pair an SRD states, with the requirement
// group it belongs to.
type example struct {
	SRD      string
	Group    string
	Markdown string `yaml:"markdown"`
	LaTeX    string `yaml:"latex"`
	Note     string `yaml:"note"`
}

// srdExamples reads every worked example in the corpus.
//
// The examples are read rather than copied, so a fixture and the specification
// it came from cannot drift apart: changing an SRD example changes what this
// test converts and what it expects, in the same edit.
func srdExamples(t *testing.T) []example {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("docs", "specs", "software-requirements", "srd*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("the corpus holds no requirement documents")
	}

	var found []example
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document struct {
			ID           string `yaml:"id"`
			Requirements map[string]struct {
				Examples []example `yaml:"examples"`
			} `yaml:"requirements"`
		}
		if err := yaml.Unmarshal(data, &document); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for group, requirements := range document.Requirements {
			for _, stated := range requirements.Examples {
				stated.SRD, stated.Group = document.ID, group
				found = append(found, stated)
			}
		}
	}
	return found
}

// chapterExamples are the SRD examples whose markdown is a chapter fragment
// the conversion path takes. The others state a title page, a roster, or a
// permitted LaTeX form, and are exercised by the tests that own them.
func chapterExamples(t *testing.T) []example {
	var chapters []example
	for _, stated := range srdExamples(t) {
		switch {
		case stated.SRD == "srd001-front-matter",
			stated.SRD == "srd008-container",
			stated.SRD == "srd009-backport-compatibility":
			continue
		case strings.HasPrefix(strings.TrimSpace(stated.Markdown), "---"):
			continue
		}
		chapters = append(chapters, stated)
	}
	return chapters
}

// TestEverySRDExampleConverts covers the corpus end to end: every worked
// example a requirement document states converts to the LaTeX that document
// states, byte for byte.
//
// This is what rel01.0 validates. The unit tests assert the same pairs, but
// each states its expectation in Go; here the expectation is read from the
// specification itself, so an SRD example nobody implemented, or an
// implementation that drifted from its example, fails without anyone
// remembering to check.
func TestEverySRDExampleConverts(t *testing.T) {
	examples := chapterExamples(t)
	if len(examples) < 8 {
		t.Fatalf("found %d chapter examples; the corpus states more than that", len(examples))
	}

	for _, stated := range examples {
		t.Run(stated.SRD+" "+stated.Group, func(t *testing.T) {
			result, err := mdtotex.Convert([]byte(stated.Markdown), stated.SRD+".md", mdtotex.Options{})
			if err != nil {
				t.Fatalf("Convert() error: %v\nmarkdown:\n%s", err, stated.Markdown)
			}
			if got := string(result.LaTeX); got != stated.LaTeX {
				t.Errorf("the example does not convert as the SRD states it\n got:\n%s\nwant:\n%s", got, stated.LaTeX)
			}
		})
	}
}

// TestTheFrontMatterExampleConverts covers the title-page example, which takes
// its own entry point rather than the chapter path.
func TestTheFrontMatterExampleConverts(t *testing.T) {
	for _, stated := range srdExamples(t) {
		if stated.SRD != "srd001-front-matter" {
			continue
		}
		t.Run(stated.Group, func(t *testing.T) {
			result, err := mdtotex.RenderFrontMatter([]byte(stated.Markdown), "00-front-matter.md", mdtotex.Options{})
			if err != nil {
				t.Fatalf("RenderFrontMatter() error: %v", err)
			}
			if got := string(result.LaTeX); got != stated.LaTeX {
				t.Errorf("the example does not render as the SRD states it\n got:\n%s\nwant:\n%s", got, stated.LaTeX)
			}
		})
	}
}
