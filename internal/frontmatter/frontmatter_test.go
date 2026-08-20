package frontmatter

import (
	"strings"
	"testing"
)

// TestReadDecodesTheMetadata covers srd001-front-matter R1.1 and R1.2: the
// fields the manuscripts state, with unknown fields ignored rather than
// rejected because Obsidian carries its own.
func TestReadDecodesTheMetadata(t *testing.T) {
	const source = `---
title: A Reference Architecture for L5 Autonomous Networks
subtitle: "Level 5 Autonomy: What It Would Take"
date: August 2026
author: Petar Djukic
tags:
  - architecture
---

Body prose.
`

	page, err := Read([]byte(source), "00-front-matter.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if page.Title != "A Reference Architecture for L5 Autonomous Networks" {
		t.Errorf("Title = %q", page.Title)
	}
	if page.Subtitle != "Level 5 Autonomy: What It Would Take" {
		t.Errorf("Subtitle = %q", page.Subtitle)
	}
	if page.Date != "August 2026" {
		t.Errorf("Date = %q", page.Date)
	}
	if string(page.Author) != "Petar Djukic" {
		t.Errorf("Author = %q", page.Author)
	}
}

// TestReadTakesTheAbstractFromEitherPlace covers srd001-front-matter R1.6,
// R1.7, R5.1, AC1, AC2, and AC6: a frontmatter field wins, a marked body run
// works, and the marker itself does not survive.
//
// Both fixtures are paperkit's own, from frontmatter_test.go, because R5.1
// holds this decoding to the behavior the manuscripts are written against.
func TestReadTakesTheAbstractFromEitherPlace(t *testing.T) {
	const marked = `---
title: A Reference Architecture
author: Petar Djukic
---

***Abstract***

This document presents a reference architecture.

It decomposes the system into two loops.
`
	page, err := Read([]byte(marked), "00-front-matter.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if strings.Contains(page.Abstract, "Abstract") {
		t.Errorf("the marker survived into the abstract:\n%s", page.Abstract)
	}
	if !strings.HasPrefix(page.Abstract, "This document presents") {
		t.Errorf("the abstract does not start at the body:\n%s", page.Abstract)
	}
	if !strings.Contains(page.Abstract, "two loops") {
		t.Errorf("the abstract dropped its later paragraphs:\n%s", page.Abstract)
	}
	if strings.Contains(page.Abstract, "tags:") || strings.Contains(page.Abstract, "title:") {
		t.Errorf("frontmatter leaked into the abstract:\n%s", page.Abstract)
	}

	const stated = `---
title: Autogenic Systems
author: Petar Djukic
abstract: |
  Advances in code-generating AI are moving software toward systems
  that evolve themselves.
---

` + "```{=latex}\n\\begin{IEEEkeywords}\nAutogenic systems, LLM agents.\n\\end{IEEEkeywords}\n```\n"

	page, err = Read([]byte(stated), "00-front-matter.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if !strings.HasPrefix(page.Abstract, "Advances in code-generating AI") {
		t.Errorf("the abstract did not come from the frontmatter: %q", page.Abstract)
	}
	if strings.Contains(page.Abstract, "IEEEkeywords") {
		t.Errorf("the keywords block was swallowed into the abstract:\n%s", page.Abstract)
	}
	if !strings.Contains(page.Body, "IEEEkeywords") {
		t.Errorf("the body lost the keywords block:\n%s", page.Body)
	}
}

// TestReadDecodesTheAuthorEitherWay covers srd001-front-matter R1.5 and AC7: a
// scalar and a list both decode, the list joining with the word and.
func TestReadDecodesTheAuthorEitherWay(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "a scalar",
			source: "---\ntitle: A Paper\nauthor: Petar Djukic\n---\n",
			want:   "Petar Djukic",
		},
		{
			name:   "a list of two",
			source: "---\ntitle: A Paper\nauthor:\n  - Petar Djukic\n  - Ada Lovelace\n---\n",
			want:   "Petar Djukic and Ada Lovelace",
		},
		{
			name:   "a list of three",
			source: "---\ntitle: A Paper\nauthor:\n  - A\n  - B\n  - C\n---\n",
			want:   "A and B and C",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			page, err := Read([]byte(testCase.source), "00-front-matter.md")
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}
			if string(page.Author) != testCase.want {
				t.Errorf("Author = %q, want %q", page.Author, testCase.want)
			}
		})
	}
}

// TestReadErrors covers srd001-front-matter R1.1, R1.3, R1.4, and AC5: a page
// with no frontmatter, a duplicate key, and a missing title each fail naming
// the file.
func TestReadErrors(t *testing.T) {
	cases := []struct {
		name   string
		source string
		detail string
	}{
		{
			name:   "no frontmatter block",
			source: "# A heading\n\nProse.\n",
			detail: "has no YAML frontmatter",
		},
		{
			name:   "a duplicate key",
			source: "---\ntitle: One\ntitle: Two\nauthor: A\n---\n",
			detail: "already defined",
		},
		{
			name:   "no title",
			source: "---\nauthor: Petar Djukic\n---\n",
			detail: "states no title",
		},
		{
			name:   "an empty title",
			source: "---\ntitle: \"   \"\nauthor: Petar Djukic\n---\n",
			detail: "states no title",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Read([]byte(testCase.source), "00-front-matter.md")
			if err == nil {
				t.Fatal("Read() accepted the page")
			}
			if !strings.Contains(err.Error(), "00-front-matter.md") {
				t.Errorf("error = %q, want it to name the file", err)
			}
			if !strings.Contains(err.Error(), testCase.detail) {
				t.Errorf("error = %q, want it to name %q", err, testCase.detail)
			}
		})
	}
}

// TestTitleBlockRendersTheSRDExample covers srd001-front-matter R2.1, R2.2,
// R2.4, and AC4 with the example the SRD states.
func TestTitleBlockRendersTheSRDExample(t *testing.T) {
	const source = `---
title: Converting Markdown to IEEEtran
subtitle: A Library for Paper Pipelines
author: Petar Djukic
---
`
	const want = "\\title{Converting Markdown to IEEEtran\\\\ \\large A Library for Paper Pipelines}\n" +
		"\\author{Petar Djukic}\n" +
		"\\maketitle\n"

	page, err := Read([]byte(source), "00-front-matter.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	block, err := page.TitleBlock()
	if err != nil {
		t.Fatalf("TitleBlock() error: %v", err)
	}
	if block != want {
		t.Errorf("TitleBlock() =\n%q\nwant\n%q", block, want)
	}
}

// TestAuthorFieldRendersTheSRDExample covers srd001-front-matter R3.1, R3.2,
// and AC3 with the example the SRD states.
func TestAuthorFieldRendersTheSRDExample(t *testing.T) {
	const source = "---\ntitle: A Paper\n" +
		"author: Petar Djukic [petar.djukic@example.com](mailto:petar.djukic@example.com)\n---\n"
	const want = "\\title{A Paper}\n" +
		"\\author{Petar Djukic\\\\ \\texttt{petar.djukic@example.com}}\n" +
		"\\maketitle\n"

	page, err := Read([]byte(source), "00-front-matter.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	block, err := page.TitleBlock()
	if err != nil {
		t.Fatalf("TitleBlock() error: %v", err)
	}
	if block != want {
		t.Errorf("TitleBlock() =\n%q\nwant\n%q", block, want)
	}
}

// TestAuthorFieldWithoutALink covers srd001-front-matter R3.3 and R3.4: a
// plain name renders alone, and a list with a link inside is treated as R3.1
// treats it.
func TestAuthorFieldWithoutALink(t *testing.T) {
	page := Page{Title: "A Paper", Author: "Petar Djukic"}
	if got := page.AuthorField(); got != "Petar Djukic" {
		t.Errorf("AuthorField() = %q", got)
	}

	page = Page{Title: "A Paper", Author: "A and B [b@example.com](mailto:b@example.com)"}
	if got := page.AuthorField(); got != `A and B\\ \texttt{b@example.com}` {
		t.Errorf("AuthorField() = %q", got)
	}

	page = Page{Title: "A Paper"}
	if got := page.AuthorField(); got != "" {
		t.Errorf("AuthorField() = %q, want empty", got)
	}
}

// TestTitleBlockOmitsTheAuthorCommand covers srd001-front-matter R2.1: the
// author command is emitted only when there is an author.
func TestTitleBlockOmitsTheAuthorCommand(t *testing.T) {
	page := Page{Title: "A Paper"}

	block, err := page.TitleBlock()
	if err != nil {
		t.Fatalf("TitleBlock() error: %v", err)
	}
	if strings.Contains(block, `\author`) {
		t.Errorf("TitleBlock() = %q, want no author command", block)
	}
	if block != "\\title{A Paper}\n\\maketitle\n" {
		t.Errorf("TitleBlock() = %q", block)
	}
}

// TestMetadataIsEscapedBeforeSubstitution covers srd001-front-matter R2.4,
// AC9, and srd003-escaping R3.3: the fields are escaped before they reach the
// template, and the template escapes nothing of its own.
func TestMetadataIsEscapedBeforeSubstitution(t *testing.T) {
	page := Page{
		Title:    "R&D on 100% of the item_one budget",
		Subtitle: "Costs $5 in {braces}",
		Author:   "A & B",
	}

	block, err := page.TitleBlock()
	if err != nil {
		t.Fatalf("TitleBlock() error: %v", err)
	}
	for _, want := range []string{
		`R\&D on 100\% of the item\_one budget`,
		`Costs \$5 in \{braces\}`,
		`\author{A \& B}`,
	} {
		if !strings.Contains(block, want) {
			t.Errorf("TitleBlock() = %q, want it to carry %q", block, want)
		}
	}
	// A template that escaped its own output would double these.
	if strings.Contains(block, `\\&`) || strings.Contains(block, "&amp;") {
		t.Errorf("the template escaped what the escaper had already escaped: %q", block)
	}
}

// TestDateIsDecodedAndNotEmitted covers srd001-front-matter R2.3 and AC8.
func TestDateIsDecodedAndNotEmitted(t *testing.T) {
	page, err := Read([]byte("---\ntitle: A Paper\ndate: August 2026\nauthor: A\n---\n"), "00-front-matter.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if page.Date != "August 2026" {
		t.Errorf("Date = %q, want it decoded", page.Date)
	}

	block, err := page.TitleBlock()
	if err != nil {
		t.Fatalf("TitleBlock() error: %v", err)
	}
	if strings.Contains(block, "August 2026") || strings.Contains(block, `\date`) {
		t.Errorf("TitleBlock() = %q, want no date", block)
	}
}

// TestTitlePageRendersNothingElse covers srd001-front-matter R5.3 and AC11:
// the reference also renders an acknowledgments file, and that stays with the
// caller. This component renders a title page and nothing else.
func TestTitlePageRendersNothingElse(t *testing.T) {
	const source = `---
title: A Paper
author: A
abstract: An abstract.
---

# Acknowledgment

The authors thank the reviewers.
`

	page, err := Read([]byte(source), "00-front-matter.md")
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	block, err := page.TitleBlock()
	if err != nil {
		t.Fatalf("TitleBlock() error: %v", err)
	}
	if strings.Contains(block, "section*") || strings.Contains(block, "Acknowledgment") {
		t.Errorf("the title block rendered an acknowledgment section: %q", block)
	}

	// The heading below the frontmatter is body, and the body is the caller's
	// to place; nothing here treats it as an acknowledgments file.
	if !strings.Contains(page.Body, "# Acknowledgment") {
		t.Errorf("the body was rewritten rather than left to the caller:\n%s", page.Body)
	}
}
