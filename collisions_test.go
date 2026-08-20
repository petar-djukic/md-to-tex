package mdtotex_test

import (
	"strings"
	"testing"

	mdtotex "github.com/petar-djukic/md-to-tex"
)

func convertChapter(t *testing.T, name, source string) mdtotex.Result {
	t.Helper()
	result, err := mdtotex.Convert([]byte(source), name, mdtotex.Options{})
	if err != nil {
		t.Fatalf("Convert(%s) error: %v", name, err)
	}
	return result
}

// TestConvertReportsItsLabels covers srd-2-renderer-core R7.1: every label the
// fragment carries, with its heading and whether the author stated it.
func TestConvertReportsItsLabels(t *testing.T) {
	const source = "# Introduction\n\nProse.\n\n## The frame {#sec:frame}\n"

	result := convertChapter(t, "01-introduction.md", source)

	if len(result.Labels) != 2 {
		t.Fatalf("Labels = %d, want 2: %+v", len(result.Labels), result.Labels)
	}
	if result.Labels[0] != (mdtotex.Label{Identifier: "introduction", Heading: "Introduction", Derived: true}) {
		t.Errorf("Labels[0] = %+v", result.Labels[0])
	}
	if result.Labels[1] != (mdtotex.Label{Identifier: "sec:frame", Heading: "The frame", Derived: false}) {
		t.Errorf("Labels[1] = %+v", result.Labels[1])
	}
	if result.Name != "01-introduction.md" {
		t.Errorf("Name = %q, want the chapter it came from", result.Name)
	}
}

// TestFailedConversionReportsNoLabels covers srd-2-renderer-core R7.2: the
// labels of a fragment that was never produced describe nothing.
func TestFailedConversionReportsNoLabels(t *testing.T) {
	const source = "# Introduction\n\nProse.\n\n---\n"

	result, err := mdtotex.Convert([]byte(source), "01-introduction.md", mdtotex.Options{})
	if err == nil {
		t.Fatal("Convert() accepted a thematic break")
	}
	if result.Labels != nil || result.LaTeX != nil {
		t.Errorf("failed conversion returned %+v, want the zero Result", result)
	}
}

// TestCollisionsNameBothChapters covers srd-2-renderer-core R7.3 and AC8: two
// chapters deriving the same identifier are reported with both chapters and
// the heading behind each, before anything is compiled.
func TestCollisionsNameBothChapters(t *testing.T) {
	first := convertChapter(t, "04-use-case-fault.md", "# Use case\n\n## Level mechanics\n")
	second := convertChapter(t, "05-use-case-change.md", "# Change management\n\n## Level mechanics\n")

	collisions := mdtotex.Collisions(first, second)

	if len(collisions) != 1 {
		t.Fatalf("Collisions() = %d, want 1: %+v", len(collisions), collisions)
	}
	collision := collisions[0]
	if collision.Identifier != "level-mechanics" {
		t.Errorf("Identifier = %q, want level-mechanics", collision.Identifier)
	}
	if len(collision.Carriers) != 2 {
		t.Fatalf("Carriers = %+v, want two", collision.Carriers)
	}
	if collision.Carriers[0].Chapter != "04-use-case-fault.md" ||
		collision.Carriers[1].Chapter != "05-use-case-change.md" {
		t.Errorf("Carriers name %q and %q", collision.Carriers[0].Chapter, collision.Carriers[1].Chapter)
	}
	for _, carrier := range collision.Carriers {
		if carrier.Heading != "Level mechanics" {
			t.Errorf("Carrier heading = %q, want the heading behind the label", carrier.Heading)
		}
	}
}

// TestCollisionsReportsNothingWhenChaptersAgree covers srd-2-renderer-core
// R7.3: distinct identifiers are not a collision.
func TestCollisionsReportsNothingWhenChaptersAgree(t *testing.T) {
	first := convertChapter(t, "01-introduction.md", "# Introduction\n\n## The frame\n")
	second := convertChapter(t, "02-literature-survey.md", "# Literature survey\n\n## Standards\n")

	if collisions := mdtotex.Collisions(first, second); len(collisions) != 0 {
		t.Errorf("Collisions() = %+v, want none", collisions)
	}
}

// TestCollisionsTreatStatedAndDerivedAlike covers srd-2-renderer-core R7.5:
// both break the same compile, so both are reported the same way.
func TestCollisionsTreatStatedAndDerivedAlike(t *testing.T) {
	stated := convertChapter(t, "03-loop.md", "# The loop {#sec:shared}\n")
	alsoStated := convertChapter(t, "07-future.md", "# Future directions {#sec:shared}\n")

	collisions := mdtotex.Collisions(stated, alsoStated)
	if len(collisions) != 1 || collisions[0].Identifier != "sec:shared" {
		t.Fatalf("Collisions() = %+v, want the stated identifier reported", collisions)
	}

	derived := convertChapter(t, "08-a.md", "# Shared heading\n")
	alsoDerived := convertChapter(t, "09-b.md", "# Shared heading\n")

	if got := mdtotex.Collisions(derived, alsoDerived); len(got) != 1 {
		t.Errorf("Collisions() on derived identifiers = %+v, want one", got)
	}
}

// TestCollisionsReadsNothingAndReturnsCollisions covers srd-2-renderer-core
// R7.4, R7.6, and AC9: a function over reports held in memory, returning
// collisions rather than an error.
func TestCollisionsReadsNothingAndReturnsCollisions(t *testing.T) {
	reports := []mdtotex.Result{
		{Name: "a.md", Labels: []mdtotex.Label{{Identifier: "intro", Heading: "Introduction", Derived: true}}},
		{Name: "b.md", Labels: []mdtotex.Label{{Identifier: "intro", Heading: "Intro", Derived: true}}},
	}

	collisions := mdtotex.Collisions(reports...)

	if len(collisions) != 1 {
		t.Fatalf("Collisions() = %+v, want one", collisions)
	}
	if collisions[0].Carriers[0].Chapter != "a.md" || collisions[0].Carriers[1].Chapter != "b.md" {
		t.Errorf("Carriers = %+v", collisions[0].Carriers)
	}
}

// TestLabelsDoNotDependOnTheirCompany covers srd-2-renderer-core R7.7 and
// AC10: no chapter stem is prefixed, so a cross-chapter reference written as
// raw LaTeX still points at the identifier the chapter emits.
func TestLabelsDoNotDependOnTheirCompany(t *testing.T) {
	const source = "# Literature survey\n\nProse.\n"

	alone := convertChapter(t, "02-literature-survey.md", source)
	alongside := convertChapter(t, "02-literature-survey.md", source)
	_ = convertChapter(t, "01-introduction.md", "# Literature survey\n")

	if string(alone.LaTeX) != string(alongside.LaTeX) {
		t.Errorf("fragment differs:\n%q\n%q", alone.LaTeX, alongside.LaTeX)
	}
	if alone.Labels[0].Identifier != "literature-survey" {
		t.Errorf("Identifier = %q, want the unprefixed slug", alone.Labels[0].Identifier)
	}
	for _, forbidden := range []string{"02-", "02:", "literature-survey:literature-survey"} {
		if strings.Contains(string(alone.LaTeX), forbidden) {
			t.Errorf("fragment carries a chapter-stem prefix %q: %s", forbidden, alone.LaTeX)
		}
	}
}

// TestCollisionsAcrossManyChapters covers srd-2-renderer-core R7.3 at the
// scale a manuscript reaches: one identifier shared by three chapters is one
// collision naming all three.
func TestCollisionsAcrossManyChapters(t *testing.T) {
	var results []mdtotex.Result
	for _, name := range []string{"04-a.md", "05-b.md", "06-c.md"} {
		results = append(results, convertChapter(t, name, "# What the agent does\n"))
	}
	results = append(results, convertChapter(t, "07-d.md", "# Future directions\n"))

	collisions := mdtotex.Collisions(results...)

	if len(collisions) != 1 {
		t.Fatalf("Collisions() = %+v, want one", collisions)
	}
	if len(collisions[0].Carriers) != 3 {
		t.Errorf("Carriers = %+v, want three", collisions[0].Carriers)
	}
}
