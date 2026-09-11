// Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT

package parse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

const note = `---
title: "Ünïcode note"
tags: [alpha, beta]
citekey: note-2024
---

Author One, Author Two
Example Org

# Ünïcode heading {#intro}

A paragraph citing [@du-2023] and linking [[notes/beta|Beta]] and [the site](https://example.org).

- one <https://auto.example.org>
- two

## References

- [@coronado-2022-ztn-survey] the survey.

` + "```latex\n\\relax\n```\n"

func TestParseReportsTheFieldsWithRuneOffsets(t *testing.T) {
	document := Parse([]byte(note), "note.md")
	if document.FrontMatter["title"] != "Ünïcode note" || document.FrontMatter["citekey"] != "note-2024" {
		t.Fatalf("front matter = %v", document.FrontMatter)
	}
	if len(document.Headings) != 2 || document.Headings[0].Identifier != "intro" || document.Headings[0].Derived ||
		document.Headings[1].Identifier != "references" || !document.Headings[1].Derived || document.Headings[0].Level != 1 {
		t.Fatalf("headings = %+v", document.Headings)
	}
	source := []rune(note)
	for _, segment := range document.Segments {
		if got := string(source[segment.Start:segment.End]); got != segment.Text {
			t.Errorf("segment %s [%d,%d) text %q does not match the source span %q", segment.Role, segment.Start, segment.End, segment.Text, got)
		}
	}
	roles := map[string]int{}
	for _, segment := range document.Segments {
		roles[segment.Role]++
	}
	if roles[RoleByline] != 1 || roles[RoleHeading] != 2 || roles[RoleReferences] != 1 || roles[RoleCode] != 1 || roles[RoleBody] != 2 {
		t.Fatalf("roles = %v; segments %+v", roles, document.Segments)
	}
	var code Segment
	for _, segment := range document.Segments {
		if segment.Role == RoleCode {
			code = segment
		}
	}
	if code.Language != "latex" || !strings.HasPrefix(code.Text, "```latex") || !strings.HasSuffix(code.Text, "```") {
		t.Fatalf("code segment = %+v", code)
	}
	if len(document.Citations) != 2 || document.Citations[0].Keys[0] != "du-2023" || document.Citations[1].Keys[0] != "coronado-2022-ztn-survey" {
		t.Fatalf("citations = %+v", document.Citations)
	}
	if got := string(source[document.Citations[0].Offset : document.Citations[0].Offset+8]); got != "[@du-202" {
		t.Errorf("citation offset points at %q", got)
	}
	kinds := map[string]Link{}
	for _, link := range document.Links {
		kinds[link.Kind] = link
	}
	if kinds[LinkWiki].Target != "notes/beta" || kinds[LinkWiki].Text != "Beta" || kinds[LinkMarkdown].Target != "https://example.org" || kinds[LinkAuto].Target != "https://auto.example.org" {
		t.Fatalf("links = %+v", document.Links)
	}
	if got := string(source[kinds[LinkWiki].Offset : kinds[LinkWiki].Offset+2]); got != "[[" {
		t.Errorf("wikilink offset points at %q", got)
	}
	if document.Runes != utf8.RuneCountInString(note) || len(document.Problems) != 0 {
		t.Fatalf("runes %d problems %v", document.Runes, document.Problems)
	}
	if !strings.Contains(document.Segments[len(document.Segments)-2].Heading, "references") {
		t.Errorf("the reference list is not attributed to its heading: %+v", document.Segments[len(document.Segments)-2])
	}
}

func TestParseSegmentsCoverTheBody(t *testing.T) {
	for _, name := range []string{"00-front-matter.md", "01-introduction.md", "02-floats.md"} {
		source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "manuscript", name))
		if err != nil {
			t.Fatal(err)
		}
		document := Parse(source, name)
		runes := []rune(string(source))
		var joined strings.Builder
		last := 0
		for _, segment := range document.Segments {
			if segment.Start < last {
				t.Errorf("%s: segment at %d starts before the previous ended at %d", name, segment.Start, last)
			}
			if string(runes[segment.Start:segment.End]) != segment.Text {
				t.Errorf("%s: segment [%d,%d) does not match its span", name, segment.Start, segment.End)
			}
			joined.WriteString(segment.Text)
			last = segment.End
		}
		body := source
		if index := strings.Index(string(source), "\n---\n"); strings.HasPrefix(string(source), "---\n") && index > 0 {
			body = source[index+5:]
		}
		if squash(joined.String()) != squash(string(body)) {
			t.Errorf("%s: the joined segments do not reproduce the body", name)
		}
	}
}

func TestParseReportsAMalformedCitationAndNoFrontMatter(t *testing.T) {
	document := Parse([]byte("Plain text with [@ broken.\n"), "plain.md")
	if len(document.FrontMatter) != 0 || len(document.Problems) != 1 || document.Problems[0].Construct != "citation" || document.Problems[0].Line != 1 {
		t.Fatalf("document = %+v", document)
	}
	if len(document.Segments) != 1 || document.Segments[0].Role != RoleBody {
		t.Fatalf("a headingless note has no byline: %+v", document.Segments)
	}
}

func squash(text string) string {
	return strings.Join(strings.Fields(text), "")
}
