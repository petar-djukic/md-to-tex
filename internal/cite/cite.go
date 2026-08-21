// Package cite parses the inline citation syntax the manuscripts use and
// renders it as a LaTeX cite command.
//
// Goldmark has no notion of the syntax: it sees a link that failed to parse
// and hands the renderer a bracketed text run, which the escaper then protects
// character by character, so the citation reaches the LaTeX as prose. This
// package supplies the inline parser that recognises it (srd006-citations R1),
// the node it produces, and the rendering and validation the renderer core
// calls (srd006-citations R2, R3).
package cite

import (
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Kind identifies the citation node in a goldmark tree.
var Kind = ast.NewNodeKind("Citation")

// Node is one bracketed citation run, holding its keys in source order
// (srd006-citations R1.3, R1.4).
type Node struct {
	ast.BaseInline

	// Keys are the citation keys, in the order written and with duplicates
	// kept: deduplication is the bibliography style's business (R1.4).
	Keys []string

	// Offset is where the run began in the source, so a caller reporting an
	// unknown key can name the line (R3.1).
	Offset int
}

// Kind reports the node kind, which is what a renderer dispatches on.
func (n *Node) Kind() ast.NodeKind { return Kind }

// Dump prints the node for goldmark's debugging output.
func (n *Node) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Keys": strings.Join(n.Keys, ",")}, nil)
}

// KeySet is the set of citation keys the caller's corpus holds.
//
// A nil KeySet turns validation off; an empty non-nil KeySet holds no valid
// key, so every citation fails against it. The two are distinguishable, which
// is what srd006-citations R3.2 requires.
type KeySet map[string]struct{}

// NewKeySet builds a KeySet from the caller's keys. The library never reads a
// reference corpus; the caller owns it (srd006-citations R3.3).
func NewKeySet(keys ...string) KeySet {
	set := make(KeySet, len(keys))
	for _, key := range keys {
		set[key] = struct{}{}
	}
	return set
}

// UnknownKeyError is a citation naming a key the caller's corpus does not
// hold. The renderer core wraps it with the file and the line, which it can
// derive from Offset (srd006-citations R3.1, R3.4).
type UnknownKeyError struct {
	Key    string
	Offset int
}

func (e *UnknownKeyError) Error() string {
	return fmt.Sprintf("unknown citation key %q", e.Key)
}

// Validate reports the first key the set does not hold, or nil when the set is
// absent or holds them all. Reporting the first rather than every failure
// names one line an author can go to (srd006-citations R3.1, R3.4).
func Validate(node *Node, keys KeySet) error {
	if keys == nil {
		return nil
	}
	for _, key := range node.Keys {
		if _, ok := keys[key]; !ok {
			return &UnknownKeyError{Key: key, Offset: node.Offset}
		}
	}
	return nil
}

// Render writes the cite command for a citation node: the keys joined by
// commas with no spaces (srd006-citations R2.1).
//
// Keys are written raw. A key that needed escaping is a key the corpus should
// not hold (srd006-citations R2.2).
func Render(writer io.Writer, node *Node) error {
	_, err := io.WriteString(writer, `\cite{`+strings.Join(node.Keys, ",")+`}`)
	return err
}

// Extension registers the citation parser with a goldmark instance.
var Extension goldmark.Extender = extension{}

type extension struct{}

// Extend registers the inline parser ahead of goldmark's link parser, which
// shares the opening-bracket trigger. Precedence is safe because the citation
// parser declines anything that is not a citation, and goldmark then offers
// the run to the parsers behind it (srd006-citations R4.1).
func (extension) Extend(markdown goldmark.Markdown) {
	markdown.Parser().AddOptions(
		parser.WithInlineParsers(util.Prioritized(&citationParser{}, 100)),
	)
}

type citationParser struct{}

// Trigger is the opening bracket, which is where a citation run starts
// (srd006-citations R1.3).
func (p *citationParser) Trigger() []byte { return []byte{'['} }

// Parse consumes one citation run and returns its node, or nil when the run is
// not a citation. Returning nil rather than an empty node is what lets link
// and image syntax parse as they always did (srd006-citations R4.1).
//
// A run that opens with a bracket and an at sign states the intent to cite, so
// failing to parse one is a defect in the source rather than a construct to
// pass along. The parser records it on the context and declines; the renderer
// core reads it and fails with the position (srd006-citations R4.2, R4.6).
func (p *citationParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, segment := block.PeekLine()
	keys, width := parseRun(line)
	if width == 0 {
		if opensCitation(line) {
			recordMalformed(pc, Malformed{
				Offset: segment.Start,
				Run:    firstLine(string(line)),
			})
		}
		return nil
	}
	node := &Node{Keys: keys, Offset: segment.Start}
	block.Advance(width)
	return node
}

// opensCitation reports whether a run states the intent to cite: an opening
// bracket followed directly by an at sign. An opening bracket alone is a link,
// an array, or prose (srd006-citations R4.1, R4.6).
func opensCitation(line []byte) bool {
	return len(line) > 1 && line[0] == '[' && line[1] == '@'
}

// firstLine trims a run to what fits an error message.
func firstLine(run string) string {
	if index := strings.IndexByte(run, '\n'); index >= 0 {
		run = run[:index]
	}
	if len(run) > 40 {
		run = run[:40] + "..."
	}
	return run
}

// parseRun reads a citation from the start of line and returns its keys and
// the byte width of the run, or a zero width when line does not open one.
//
// The grammar is small enough to read directly: an opening bracket, then keys
// each introduced by an at sign and separated by a semicolon with optional
// whitespace, then a closing bracket (srd006-citations R1.1).
func parseRun(line []byte) ([]string, int) {
	if len(line) == 0 || line[0] != '[' {
		return nil, 0
	}

	var keys []string
	position := 1
	for {
		if position >= len(line) || line[position] != '@' {
			return nil, 0
		}
		position++

		start := position
		for position < len(line) {
			character, width := utf8.DecodeRune(line[position:])
			if !isKeyRune(character) {
				break
			}
			position += width
		}
		key := trimSentencePunctuation(string(line[start:position]))
		if key == "" {
			// An at sign with no key text is not a citation (R4.2).
			return nil, 0
		}
		keys = append(keys, key)

		// The key may have given back a trailing period or colon, which sits
		// between here and whatever ends the run.
		position = start + len(key)
		for position < len(line) && (line[position] == '.' || line[position] == ':') {
			position++
		}

		switch {
		case position < len(line) && line[position] == ']':
			return keys, position + 1
		case position < len(line) && line[position] == ';':
			position++
			for position < len(line) && (line[position] == ' ' || line[position] == '\t') {
				position++
			}
		default:
			// Anything else — including the end of the line, which is an
			// unterminated run — is not a citation (R4.2).
			return nil, 0
		}
	}
}

// isKeyRune reports whether a character may appear in key text: letters and
// digits in any script, and hyphens, underscores, colons, periods, and slashes
// (srd006-citations R1.2).
//
// Letters are Unicode letters rather than ASCII ones. The corpus generates
// keys from author surnames, so an accented surname produces a key carrying an
// accented letter; BibTeX resolves those, and reading keys byte by byte was
// what dropped them.
func isKeyRune(character rune) bool {
	switch character {
	case '-', '_', ':', '.', '/':
		return true
	}
	return unicode.IsLetter(character) || unicode.IsDigit(character)
}

// trimSentencePunctuation gives back a trailing period or colon, which belongs
// to the sentence rather than the key (srd006-citations R1.2).
func trimSentencePunctuation(key string) string {
	return strings.TrimRight(key, ".:")
}
