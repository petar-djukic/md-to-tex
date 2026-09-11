// Copyright (c) 2026 Petar Djukic. All rights reserved. SPDX-License-Identifier: MIT

// Package parse walks one markdown document with the same goldmark dialect
// the renderer uses and reports what a corpus importer needs rather than
// LaTeX: the front matter as parsed, the headings, the segments with their
// positions, the citations, the links, and the problems the citation
// extension records (srd010-service R2).
//
// Every position is a rune offset into the source as the caller sent it,
// front matter included, never a byte offset: a caller that stores text and
// later cites a span counts characters, and a multibyte character must not
// shift the span.
package parse

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"

	"github.com/petar-djukic/md-to-tex/internal/cite"
	"github.com/petar-djukic/md-to-tex/internal/frontmatter"
	"github.com/petar-djukic/md-to-tex/internal/render"
)

// Document is what one parse reports.
type Document struct {
	Name        string         `json:"name"`
	FrontMatter map[string]any `json:"front_matter"`
	Headings    []Heading      `json:"headings"`
	Segments    []Segment      `json:"segments"`
	Citations   []Citation     `json:"citations"`
	Links       []Link         `json:"links"`
	Problems    []Problem      `json:"problems"`
	// Runes is the length of the source in runes, so a caller can check
	// that the last segment ends inside it.
	Runes int `json:"runes"`
}

// Heading is one heading: its level, its text, the identifier the renderer
// would label it with, and where it starts.
type Heading struct {
	Level      int    `json:"level"`
	Text       string `json:"text"`
	Identifier string `json:"identifier"`
	Derived    bool   `json:"derived"`
	Offset     int    `json:"offset"`
}

// The roles a segment takes (srd010-service R2.3).
const (
	RoleHeading    = "heading"
	RoleBody       = "body"
	RoleByline     = "byline"
	RoleReferences = "references"
	RoleCode       = "code"
)

// Segment is one top-level block of the body, with the exact source text it
// spans and the identifier of the heading it sits under.
type Segment struct {
	Role    string `json:"role"`
	Text    string `json:"text"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Heading string `json:"heading,omitempty"`
	// Language is the fence's info string on a code segment.
	Language string `json:"language,omitempty"`
}

// Citation is one bracketed citation run, with its keys in the order written.
type Citation struct {
	Keys   []string `json:"keys"`
	Offset int      `json:"offset"`
}

// The kinds a link takes.
const (
	LinkWiki     = "wikilink"
	LinkMarkdown = "link"
	LinkAuto     = "autolink"
)

// Link is one wikilink, markdown link, or autolink.
type Link struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Text   string `json:"text"`
	Offset int    `json:"offset"`
}

// Problem is something the parse noticed and did not stop for.
type Problem struct {
	Construct string `json:"construct"`
	Line      int    `json:"line"`
	Offset    int    `json:"offset"`
	Detail    string `json:"detail"`
}

// The dialect is the renderer's: the citation extension and pipe tables, with
// attributes on headings. Keeping one dialect is what lets a caller trust
// that what parses here converts there.
var markdown = goldmark.New(
	goldmark.WithExtensions(cite.Extension, extension.Table),
	goldmark.WithParserOptions(parser.WithAttribute()),
)

// referencesHeading names the headings whose blocks are references rather
// than body: a bibliography, a source list, or the works cited.
var referencesHeading = regexp.MustCompile(`(?i)^(references?|bibliography|sources?|works cited|further reading|see also)\b`)

// wikilink matches Obsidian's [[target]] and [[target|alias]], and the
// [[target#heading]] form, which keeps its fragment in the target.
var wikilink = regexp.MustCompile(`\[\[([^\[\]|]+)(?:\|([^\[\]]*))?\]\]`)

// Parse reads one document. It never fails: a document with no front matter
// parses with an empty mapping, and a malformed construct is reported as a
// problem rather than an error, because an importer wants what it can get
// from every file and a list of what it could not (srd010-service R2.6).
func Parse(source []byte, name string) Document {
	document := Document{Name: name, FrontMatter: map[string]any{}, Runes: utf8.RuneCount(source)}
	runes := newRuneIndex(source)

	block, body, offset, found := frontmatter.Split(source)
	if found {
		if err := yaml.Unmarshal(block, &document.FrontMatter); err != nil {
			document.FrontMatter = map[string]any{}
			document.Problems = append(document.Problems, Problem{Construct: "front matter", Line: 1, Offset: 0, Detail: err.Error()})
		}
		if document.FrontMatter == nil {
			document.FrontMatter = map[string]any{}
		}
	} else {
		body = source
		offset = 0
	}
	// The body is parsed in place, at its byte offset within the source, so
	// every position the walk reads is a source position.
	padded := make([]byte, offset, offset+len(body))
	for i := range padded {
		padded[i] = ' '
	}
	for i := 0; i < offset; i++ {
		if source[i] == '\n' {
			padded[i] = '\n'
		}
	}
	padded = append(padded, body...)

	context := parser.NewContext()
	root := markdown.Parser().Parse(text.NewReader(padded), parser.WithContext(context))
	if malformed, ok := cite.MalformedIn(context); ok {
		document.Problems = append(document.Problems, Problem{
			Construct: "citation", Line: lineAt(padded, malformed.Offset), Offset: runes.at(malformed.Offset), Detail: malformed.Error(),
		})
	}

	walker := &walker{source: padded, runes: runes, document: &document}
	walker.blocks(root)
	walker.finish()
	sort.SliceStable(document.Links, func(i, j int) bool { return document.Links[i].Offset < document.Links[j].Offset })
	sort.SliceStable(document.Citations, func(i, j int) bool { return document.Citations[i].Offset < document.Citations[j].Offset })
	if document.Headings == nil {
		document.Headings = []Heading{}
	}
	if document.Segments == nil {
		document.Segments = []Segment{}
	}
	if document.Citations == nil {
		document.Citations = []Citation{}
	}
	if document.Links == nil {
		document.Links = []Link{}
	}
	if document.Problems == nil {
		document.Problems = []Problem{}
	}
	return document
}

type walker struct {
	source   []byte
	runes    runeIndex
	document *Document
	// current is the identifier of the heading the blocks now sit under, and
	// inReferences whether that heading names a reference list.
	current      string
	inReferences bool
	seenHeading  bool
	// leading holds the segments before the first heading, which become
	// bylines when a heading follows them (R2.4).
	leading []int
}

func (w *walker) blocks(node ast.Node) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		w.block(child)
	}
}

func (w *walker) block(node ast.Node) {
	start, end, ok := w.span(node)
	if !ok {
		return
	}
	switch typed := node.(type) {
	case *ast.Heading:
		w.heading(typed, start, end)
		return
	case *ast.FencedCodeBlock:
		language := ""
		if info := typed.Info; info != nil {
			language = strings.TrimSpace(string(info.Segment.Value(w.source)))
		}
		w.add(Segment{Role: RoleCode, Language: language}, start, end)
		return
	case *ast.CodeBlock:
		w.add(Segment{Role: RoleCode}, start, end)
		return
	case *east.Table:
		w.add(Segment{Role: w.role()}, start, end)
		return
	}
	w.add(Segment{Role: w.role()}, start, end)
	w.inlines(node)
	w.wikilinks(start, end)
}

func (w *walker) heading(node *ast.Heading, start, end int) {
	textOf := string(node.Text(w.source))
	identifier, derived := render.HeadingIdentifier(node, textOf)
	w.document.Headings = append(w.document.Headings, Heading{
		Level: node.Level, Text: textOf, Identifier: identifier, Derived: derived, Offset: w.runes.at(start),
	})
	if !w.seenHeading {
		for _, index := range w.leading {
			w.document.Segments[index].Role = RoleByline
		}
		w.leading = nil
		w.seenHeading = true
	}
	w.current = identifier
	w.inReferences = referencesHeading.MatchString(strings.TrimSpace(textOf))
	w.add(Segment{Role: RoleHeading}, start, end)
	w.inlines(node)
}

func (w *walker) role() string {
	if w.inReferences {
		return RoleReferences
	}
	return RoleBody
}

func (w *walker) add(segment Segment, start, end int) {
	segment.Text = string(w.source[start:end])
	segment.Start = w.runes.at(start)
	segment.End = w.runes.at(end)
	if segment.Role != RoleHeading {
		segment.Heading = w.current
	}
	if !w.seenHeading && segment.Role != RoleHeading {
		w.leading = append(w.leading, len(w.document.Segments))
	}
	w.document.Segments = append(w.document.Segments, segment)
}

// finish leaves the leading segments as body when no heading ever followed
// them: a note with no headings has no byline.
func (w *walker) finish() { w.leading = nil }

// span is the byte range a block covers, from its first line to the end of
// its last, trimmed of trailing whitespace so a segment ends on content. A
// container block (a list, a quote) covers its children.
func (w *walker) span(node ast.Node) (int, int, bool) {
	start, end := -1, -1
	var visit func(n ast.Node)
	visit = func(n ast.Node) {
		if n.Type() == ast.TypeBlock {
			lines := n.Lines()
			for i := 0; i < lines.Len(); i++ {
				line := lines.At(i)
				if start < 0 || line.Start < start {
					start = line.Start
				}
				if line.Stop > end {
					end = line.Stop
				}
			}
		}
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			visit(child)
		}
	}
	visit(node)
	if start < 0 {
		return 0, 0, false
	}
	// Walk back to the start of the line, so a heading keeps its # marker
	// and a list item keeps its bullet.
	for start > 0 && w.source[start-1] != '\n' {
		start--
	}
	for end > start && (w.source[end-1] == '\n' || w.source[end-1] == ' ' || w.source[end-1] == '\t' || w.source[end-1] == '\r') {
		end--
	}
	// A block's Lines can stop short of its last line's end: an ATX heading's
	// exclude its {#id} attribute and a table row's its closing pipe. The
	// block owns the rest of that line, so extend to it.
	if newline := bytes.IndexByte(w.source[end:], '\n'); newline >= 0 {
		end += newline
	} else {
		end = len(w.source)
	}
	for end > start && (w.source[end-1] == ' ' || w.source[end-1] == '\t' || w.source[end-1] == '\r') {
		end--
	}
	// A fenced code block's Lines exclude its fences; extend to the closing
	// fence line so the segment text is what the author wrote.
	if fenced, ok := node.(*ast.FencedCodeBlock); ok {
		if opening := w.fenceStart(fenced); opening >= 0 {
			start = opening
		}
		if closing := w.fenceEnd(fenced, end); closing > end {
			end = closing
		}
	}
	return start, end, true
}

func (w *walker) fenceStart(node *ast.FencedCodeBlock) int {
	lines := node.Lines()
	if lines.Len() == 0 {
		if node.Info != nil {
			return lineStart(w.source, node.Info.Segment.Start)
		}
		return -1
	}
	first := lineStart(w.source, lines.At(0).Start)
	if first == 0 {
		return -1
	}
	return lineStart(w.source, first-1)
}

func (w *walker) fenceEnd(node *ast.FencedCodeBlock, end int) int {
	rest := w.source[end:]
	next := bytes.IndexByte(rest, '\n')
	if next < 0 {
		return end
	}
	lineEnd := end + next + 1
	if lineEnd < len(w.source) {
		if following := bytes.IndexByte(w.source[lineEnd:], '\n'); following >= 0 {
			line := w.source[lineEnd : lineEnd+following]
			if bytes.HasPrefix(bytes.TrimSpace(line), []byte("```")) || bytes.HasPrefix(bytes.TrimSpace(line), []byte("~~~")) {
				return lineEnd + following
			}
		} else if line := w.source[lineEnd:]; bytes.HasPrefix(bytes.TrimSpace(line), []byte("```")) || bytes.HasPrefix(bytes.TrimSpace(line), []byte("~~~")) {
			return len(w.source)
		}
	}
	return end
}

func lineStart(source []byte, offset int) int {
	for offset > 0 && source[offset-1] != '\n' {
		offset--
	}
	return offset
}

// inlines collects citations and links from a block's inline children,
// walking containers so a list item's links count.
func (w *walker) inlines(node ast.Node) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch typed := child.(type) {
		case *cite.Node:
			w.document.Citations = append(w.document.Citations, Citation{Keys: append([]string(nil), typed.Keys...), Offset: w.runes.at(typed.Offset)})
		case *ast.Link:
			w.document.Links = append(w.document.Links, Link{
				Kind: LinkMarkdown, Target: string(typed.Destination), Text: string(typed.Text(w.source)), Offset: w.runes.at(w.inlineOffset(typed)),
			})
		case *ast.AutoLink:
			w.document.Links = append(w.document.Links, Link{
				Kind: LinkAuto, Target: string(typed.URL(w.source)), Text: string(typed.Label(w.source)), Offset: w.runes.at(w.inlineOffset(typed)),
			})
		}
		w.inlines(child)
	}
}

// wikilinks scans a block's source span for Obsidian links, which the
// dialect has no parser for: goldmark splits the brackets across text nodes,
// so the scan runs over the span rather than node by node.
func (w *walker) wikilinks(start, end int) {
	content := w.source[start:end]
	for _, match := range wikilink.FindAllSubmatchIndex(content, -1) {
		target := string(content[match[2]:match[3]])
		alias := target
		if match[4] >= 0 {
			alias = string(content[match[4]:match[5]])
		}
		w.document.Links = append(w.document.Links, Link{
			Kind: LinkWiki, Target: target, Text: alias, Offset: w.runes.at(start + match[0]),
		})
	}
}

// inlineOffset is where an inline node's first text starts, or the enclosing
// block's start when it holds no text.
func (w *walker) inlineOffset(node ast.Node) int {
	var first func(n ast.Node) (int, bool)
	first = func(n ast.Node) (int, bool) {
		if t, ok := n.(*ast.Text); ok {
			return t.Segment.Start, true
		}
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if offset, ok := first(child); ok {
				return offset, true
			}
		}
		return 0, false
	}
	if offset, ok := first(node); ok {
		// A link's text starts after its opening bracket.
		if _, isLink := node.(*ast.Link); isLink && offset > 0 && w.source[offset-1] == '[' {
			return offset - 1
		}
		return offset
	}
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		if parent.Type() == ast.TypeBlock && parent.Lines().Len() > 0 {
			return parent.Lines().At(0).Start
		}
	}
	return 0
}

func lineAt(source []byte, offset int) int {
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + bytes.Count(source[:offset], []byte{'\n'})
}

// runeIndex maps byte offsets to rune offsets in one pass over the source.
type runeIndex []int

func newRuneIndex(source []byte) runeIndex {
	index := make(runeIndex, len(source)+1)
	count := 0
	for i := 0; i < len(source); {
		_, width := utf8.DecodeRune(source[i:])
		for j := 0; j < width; j++ {
			index[i+j] = count
		}
		i += width
		count++
	}
	index[len(source)] = count
	return index
}

func (r runeIndex) at(byteOffset int) int {
	if byteOffset < 0 {
		return 0
	}
	if byteOffset >= len(r) {
		return r[len(r)-1]
	}
	return r[byteOffset]
}
