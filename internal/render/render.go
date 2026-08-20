// Package render walks a parsed markdown chapter and writes the LaTeX
// fragment for it.
//
// The walk owns dispatch and the output buffer, and every text node reaches
// that buffer through the escaper or through raw content that declares itself
// (srd002-renderer-core R6.2). A construct outside the mapping is an error
// naming its position rather than a silent omission, which is what keeps
// markdown trustworthy as the source of truth (R6.4).
package render

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/petar-djukic/md-to-tex/internal/cite"
)

// Config is what the walk needs from the caller's options
// (srd002-renderer-core R2.1, R2.2).
type Config struct {
	// Citations is the set of keys a citation may name. A nil set turns
	// validation off; an empty set holds no valid key (srd006-citations R3.2).
	Citations cite.KeySet
}

// Label is one identifier the fragment carries, with the heading it came from
// and whether the author stated it.
type Label struct {
	Identifier string
	Heading    string
	Derived    bool
}

// Error is a conversion failure, naming the source, the line, and the
// construct that failed (srd002-renderer-core R1.3).
type Error struct {
	Name      string
	Line      int
	Construct string
	Detail    string
}

func (e *Error) Error() string {
	message := fmt.Sprintf("%s:%d: %s", e.Name, e.Line, e.Construct)
	if e.Detail != "" {
		message += ": " + e.Detail
	}
	return message
}

// markdown is the parser the walk reads from. Attributes are enabled because a
// heading may state its identifier (srd002-renderer-core R3.4); the table
// extension is enabled so a pipe table arrives as a table node and is reported
// as unmapped, rather than converting silently as prose (R6.4).
var markdown = goldmark.New(
	goldmark.WithExtensions(cite.Extension, extension.Table),
	goldmark.WithParserOptions(parser.WithAttribute()),
)

// Convert renders one chapter of markdown as a LaTeX fragment. The name is
// used only in error messages; no file is opened (srd002-renderer-core R1.1).
//
// The fragment carries no preamble and no document environment (R1.4), ends
// with exactly one newline, and does not begin with a blank line (R1.5).
// Conversion holds no state between calls, so it is deterministic and safe to
// call concurrently (R1.2).
func Convert(source []byte, name string, config Config) ([]byte, []Label, error) {
	walker := &walker{source: source, name: name, config: config, seen: map[string]string{}}

	document := markdown.Parser().Parse(text.NewReader(source))
	if err := walker.blocks(document); err != nil {
		return nil, nil, err
	}

	fragment := strings.TrimLeft(walker.out.String(), "\n")
	fragment = strings.TrimRight(fragment, "\n")
	if fragment != "" {
		fragment += "\n"
	}
	return []byte(fragment), walker.labels, nil
}

type walker struct {
	source []byte
	name   string
	config Config

	out    strings.Builder
	labels []Label
	// rawUntil is how far into the source a control sequence has already been
	// written, for the runs goldmark splits across text nodes.
	rawUntil int
	// seen maps an identifier to the heading that claimed it, so a collision
	// names both (srd002-renderer-core R3.6).
	seen map[string]string
}

// fail builds a positioned error. The line is derived from the byte offset,
// counting the newlines before it.
func (w *walker) fail(offset int, construct, detail string) error {
	if offset > len(w.source) {
		offset = len(w.source)
	}
	line := 1 + bytes.Count(w.source[:offset], []byte{'\n'})
	return &Error{Name: w.name, Line: line, Construct: construct, Detail: detail}
}

// offsetOf reports where a node starts in the source, for error positions.
//
// Some blocks carry no line segments at all - a thematic break is the case
// that matters, since it is one of the constructs R6.4 reports - so the
// fallback resumes after the previous sibling and skips the blank lines
// between them. Without it those errors all name line 1, which sends an
// author to the wrong end of the chapter.
func (w *walker) offsetOf(node ast.Node) int {
	if node.Type() == ast.TypeBlock && node.Lines().Len() > 0 {
		return node.Lines().At(0).Start
	}
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if text, ok := child.(*ast.Text); ok {
			return text.Segment.Start
		}
	}
	if previous := node.PreviousSibling(); previous != nil {
		if lines := previous.Lines(); lines.Len() > 0 {
			return firstContentAfter(w.source, lines.At(lines.Len()-1).Stop)
		}
	}
	if parent := node.Parent(); parent != nil && parent.Lines().Len() > 0 {
		return parent.Lines().At(0).Start
	}
	return 0
}

// firstContentAfter returns the start of the first line after offset that
// holds something other than whitespace.
func firstContentAfter(source []byte, offset int) int {
	position := offset
	for position < len(source) {
		lineEnd := bytes.IndexByte(source[position:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source) - position
		}
		if len(bytes.TrimSpace(source[position:position+lineEnd])) > 0 {
			return position
		}
		position += lineEnd + 1
	}
	return offset
}

// blocks renders every child of node in order.
func (w *walker) blocks(node ast.Node) error {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if err := w.block(child); err != nil {
			return err
		}
	}
	return nil
}

// block dispatches one block node (srd002-renderer-core R6.1, R6.4).
func (w *walker) block(node ast.Node) error {
	switch typed := node.(type) {
	case *ast.Heading:
		return w.heading(typed)
	case *ast.Paragraph:
		return w.paragraph(typed)
	case *ast.TextBlock:
		// A list item's own content arrives as a text block (R5.5).
		if err := w.inlines(typed); err != nil {
			return err
		}
		w.out.WriteString("\n")
		return nil
	case *ast.List:
		return w.list(typed)
	case *ast.Blockquote:
		return w.blockquote(typed)
	case *ast.FencedCodeBlock:
		return w.fencedCode(typed)
	case *ast.CodeBlock:
		return w.verbatim(codeText(typed, w.source))
	case *ast.HTMLBlock:
		return w.htmlBlock(typed)
	case *ast.ThematicBreak:
		return w.fail(w.offsetOf(typed), "thematic break", "no mapping; write it as raw LaTeX")
	case *east.Table:
		return w.fail(w.offsetOf(typed), "table", "tables render from rel00.2 (srd005-tables)")
	}
	return w.fail(w.offsetOf(node), node.Kind().String(), "no mapping for this construct")
}

// heading renders a sectioning command and the label that lets a raw
// reference resolve (srd002-renderer-core R3.1 through R3.7).
func (w *walker) heading(node *ast.Heading) error {
	command, ok := sectioningCommand(node.Level)
	if !ok {
		return w.fail(w.offsetOf(node), fmt.Sprintf("heading level %d", node.Level),
			"IEEEtran has no command below paragraph")
	}

	rendered, err := w.inlineString(node)
	if err != nil {
		return err
	}

	heading := string(node.Text(w.source))
	identifier, derived := headingIdentifier(node, heading)
	if identifier == "" {
		return w.fail(w.offsetOf(node), "heading", "its text yields no identifier; state one as {#id}")
	}
	if previous, clash := w.seen[identifier]; clash {
		return w.fail(w.offsetOf(node), "heading identifier "+identifier,
			fmt.Sprintf("already claimed by %q; %q needs its own", previous, heading))
	}
	w.seen[identifier] = heading
	w.labels = append(w.labels, Label{Identifier: identifier, Heading: heading, Derived: derived})

	w.out.WriteString(command + "{" + rendered + `}\label{` + identifier + "}\n\n")
	return nil
}

// headingIdentifier returns the identifier the author stated, or the one
// derived from the heading text (srd002-renderer-core R3.4, R3.5).
func headingIdentifier(node *ast.Heading, heading string) (string, bool) {
	if attribute, ok := node.AttributeString("id"); ok {
		switch stated := attribute.(type) {
		case []byte:
			return string(stated), false
		case string:
			return stated, false
		}
	}
	return slug(heading), true
}

// paragraph renders inline content followed by a blank line
// (srd002-renderer-core R4.1).
func (w *walker) paragraph(node *ast.Paragraph) error {
	if err := w.inlines(node); err != nil {
		return err
	}
	w.out.WriteString("\n\n")
	return nil
}

// list renders itemize or enumerate, one item command per item
// (srd002-renderer-core R5.1, R5.2, R5.5).
func (w *walker) list(node *ast.List) error {
	environment := "itemize"
	if node.IsOrdered() {
		environment = "enumerate"
	}

	w.out.WriteString(`\begin{` + environment + "}\n")
	for item := node.FirstChild(); item != nil; item = item.NextSibling() {
		w.out.WriteString(`\item `)
		if err := w.blocks(item); err != nil {
			return err
		}
	}
	w.closeEnvironment(environment)
	return nil
}

// closeEnvironment ends an environment, leaving one newline between the last
// block inside it and the closing command. Block renderers end with a blank
// line so paragraphs separate, and that blank line has no business sitting
// inside an environment about to close.
func (w *walker) closeEnvironment(environment string) {
	trimmed := strings.TrimRight(w.out.String(), "\n")
	w.out.Reset()
	w.out.WriteString(trimmed)
	w.out.WriteString("\n" + `\end{` + environment + "}\n\n")
}

// blockquote renders the quote environment (srd002-renderer-core R5.3).
func (w *walker) blockquote(node *ast.Blockquote) error {
	w.out.WriteString("\\begin{quote}\n")
	if err := w.blocks(node); err != nil {
		return err
	}
	w.closeEnvironment("quote")
	return nil
}

// verbatim writes a code block's text unescaped and unmodified
// (srd002-renderer-core R5.4).
func (w *walker) verbatim(body string) error {
	w.out.WriteString("\\begin{verbatim}\n")
	w.out.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		w.out.WriteString("\n")
	}
	w.out.WriteString("\\end{verbatim}\n\n")
	return nil
}

// htmlBlock drops a comment and reports anything else
// (srd002-renderer-core R6.3).
func (w *walker) htmlBlock(node *ast.HTMLBlock) error {
	if node.HTMLBlockType == ast.HTMLBlockType2 {
		return nil
	}
	return w.fail(w.offsetOf(node), "raw HTML", "only comments are dropped; LaTeX is written as raw LaTeX")
}

// codeText joins a code block's lines from the source.
func codeText(node ast.Node, source []byte) string {
	var builder strings.Builder
	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		builder.Write(segment.Value(source))
	}
	return builder.String()
}
