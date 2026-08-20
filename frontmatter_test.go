package mdtotex_test

import (
	"strings"
	"testing"

	mdtotex "github.com/petar-djukic/md-to-tex"
)

// TestRenderFrontMatterRendersTheSRDExample covers srd001-front-matter R4.1,
// R4.3, R4.4, R4.5, and AC1 with the example the SRD states: the abstract
// environment before the keywords block, and the keywords unescaped.
func TestRenderFrontMatterRendersTheSRDExample(t *testing.T) {
	const source = `---
title: Autogenic Systems
author: Petar Djukic
abstract: |
  Advances in code-generating AI are moving software toward
  systems that evolve themselves.
---

` + "```{=latex}\n\\begin{IEEEkeywords}\nAutogenic systems, LLM agents.\n\\end{IEEEkeywords}\n```\n"

	const want = "\\title{Autogenic Systems}\n" +
		"\\author{Petar Djukic}\n" +
		"\\maketitle\n" +
		"\\begin{abstract}\n" +
		"Advances in code-generating AI are moving software toward\n" +
		"systems that evolve themselves.\n" +
		"\\end{abstract}\n" +
		"\\begin{IEEEkeywords}\n" +
		"Autogenic systems, LLM agents.\n" +
		"\\end{IEEEkeywords}\n"

	result, err := mdtotex.RenderFrontMatter([]byte(source), "00-front-matter.md", mdtotex.Options{})
	if err != nil {
		t.Fatalf("RenderFrontMatter() error: %v", err)
	}
	if got := string(result.LaTeX); got != want {
		t.Errorf("RenderFrontMatter() =\n%q\nwant\n%q", got, want)
	}
}

// TestAbstractTakesTheChapterPath covers srd001-front-matter R4.1 and R5.2:
// the abstract converts rather than escaping, so the paper's own emphasis and
// citations survive. This is the one knowing difference from paperkit, which
// converts the abstract through pandoc.
func TestAbstractTakesTheChapterPath(t *testing.T) {
	const source = `---
title: A Paper
author: A
abstract: |
  R&D on *guided* agents, as surveyed in [@du-2023].
---
`

	result, err := mdtotex.RenderFrontMatter([]byte(source), "00-front-matter.md", mdtotex.Options{})
	if err != nil {
		t.Fatalf("RenderFrontMatter() error: %v", err)
	}
	got := string(result.LaTeX)
	if !strings.Contains(got, `R\&D on \emph{guided} agents, as surveyed in \cite{du-2023}.`) {
		t.Errorf("the abstract did not take the chapter path:\n%s", got)
	}
}

// TestNoAbstractEmitsNoEnvironment covers srd001-front-matter R4.2 and AC10.
func TestNoAbstractEmitsNoEnvironment(t *testing.T) {
	const source = "---\ntitle: A Paper\nauthor: A\nabstract: \"\"\n---\n"

	result, err := mdtotex.RenderFrontMatter([]byte(source), "00-front-matter.md", mdtotex.Options{})
	if err != nil {
		t.Fatalf("RenderFrontMatter() error: %v", err)
	}
	got := string(result.LaTeX)
	if strings.Contains(got, "abstract") {
		t.Errorf("an empty abstract emitted an environment:\n%s", got)
	}
	if got != "\\title{A Paper}\n\\author{A}\n\\maketitle\n" {
		t.Errorf("RenderFrontMatter() = %q", got)
	}
}

// TestFrontMatterFragmentShape covers srd001-front-matter R4.4 and R4.5: the
// fixed order, one newline at the end, and no preamble.
func TestFrontMatterFragmentShape(t *testing.T) {
	const source = `---
title: A Paper
author: A
abstract: An abstract.
---

Body prose.
`

	result, err := mdtotex.RenderFrontMatter([]byte(source), "00-front-matter.md", mdtotex.Options{})
	if err != nil {
		t.Fatalf("RenderFrontMatter() error: %v", err)
	}
	got := string(result.LaTeX)

	title := strings.Index(got, `\title{`)
	abstract := strings.Index(got, `\begin{abstract}`)
	body := strings.Index(got, "Body prose.")
	if !(title < abstract && abstract < body) {
		t.Errorf("the blocks are out of order:\n%s", got)
	}
	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("the fragment does not end with exactly one newline: %q", got)
	}
	for _, forbidden := range []string{`\documentclass`, `\begin{document}`, `\usepackage`} {
		if strings.Contains(got, forbidden) {
			t.Errorf("the fragment carries %s", forbidden)
		}
	}
}

// TestFrontMatterReportsAFailedConversion covers srd001-front-matter R4.1 and
// R4.3: a body carrying an unmapped construct fails rather than emitting a
// fragment with a hole in it.
func TestFrontMatterReportsAFailedConversion(t *testing.T) {
	const source = "---\ntitle: A Paper\nauthor: A\nabstract: An abstract.\n---\n\n---\n"

	result, err := mdtotex.RenderFrontMatter([]byte(source), "00-front-matter.md", mdtotex.Options{})
	if err == nil {
		t.Fatal("RenderFrontMatter() accepted a body with a thematic break")
	}
	if result.LaTeX != nil {
		t.Errorf("a failed render returned %q", result.LaTeX)
	}
}

// TestFrontMatterCitationsValidate covers srd001-front-matter R4.1 with
// srd006-citations R3.1: the abstract goes through the chapter path, so a key
// outside the caller's corpus fails there too.
func TestFrontMatterCitationsValidate(t *testing.T) {
	const source = "---\ntitle: A Paper\nauthor: A\nabstract: Cited [@absent-key].\n---\n"

	_, err := mdtotex.RenderFrontMatter([]byte(source), "00-front-matter.md",
		mdtotex.Options{CitationKeys: []string{"du-2023"}})
	if err == nil {
		t.Fatal("RenderFrontMatter() accepted an unknown citation key")
	}
	if !strings.Contains(err.Error(), "absent-key") {
		t.Errorf("error = %q, want it to name the key", err)
	}
}
