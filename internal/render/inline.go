package render

import (
	"strings"

	"github.com/yuin/goldmark/ast"

	"github.com/petar-djukic/md-to-tex/internal/cite"
	"github.com/petar-djukic/md-to-tex/internal/latex"
)

// inlines renders the inline children of a block node into the output buffer.
func (w *walker) inlines(node ast.Node) error {
	rendered, err := w.inlineString(node)
	if err != nil {
		return err
	}
	w.out.WriteString(rendered)
	return nil
}

// inlineString renders inline children into a string, for the places that need
// the rendered content before writing it - a heading argument, for instance.
func (w *walker) inlineString(node ast.Node) (string, error) {
	var builder strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if err := w.inline(&builder, child); err != nil {
			return "", err
		}
	}
	return builder.String(), nil
}

// inline dispatches one inline node. Text reaches the builder through the
// escaper, and raw content declares itself (srd002-renderer-core R6.2).
func (w *walker) inline(builder *strings.Builder, node ast.Node) error {
	switch typed := node.(type) {
	case *ast.Text:
		return w.text(builder, typed)
	case *ast.String:
		builder.WriteString(latex.Escape(string(typed.Value)))
		return nil
	case *ast.Emphasis:
		return w.emphasis(builder, typed)
	case *ast.CodeSpan:
		builder.WriteString(`\texttt{` + latex.Escape(string(typed.Text(w.source))) + `}`)
		return nil
	case *ast.Link:
		return w.link(builder, typed)
	case *ast.AutoLink:
		builder.WriteString(`\url{` + string(typed.URL(w.source)) + `}`)
		return nil
	case *cite.Node:
		return w.citation(builder, typed)
	case *ast.RawHTML:
		return w.rawHTML(typed)
	case *ast.Image:
		return w.fail(w.offsetOf(typed), "image", "figures render from rel00.2 (srd004-figures)")
	}
	return w.fail(w.offsetOf(node), node.Kind().String(), "no mapping for this construct")
}

// text renders a text node - escaping its prose, passing raw LaTeX through -
// and carries its line breaks (srd002-renderer-core R4.1, R4.4).
func (w *walker) text(builder *strings.Builder, node *ast.Text) error {
	start, stop := node.Segment.Start, node.Segment.Stop
	if w.rawUntil > start {
		// A control sequence in an earlier node reached into this one and was
		// written whole (srd007-passthrough R2.1).
		if w.rawUntil >= stop {
			w.writeLineBreak(builder, node)
			return nil
		}
		start = w.rawUntil
	}

	content := string(w.source[start:stop])
	if err := w.writeText(builder, content, start, w.blockEnd(node)); err != nil {
		return err
	}
	w.writeLineBreak(builder, node)
	return nil
}

// writeLineBreak carries a text node's line break (srd002-renderer-core R4.4).
func (w *walker) writeLineBreak(builder *strings.Builder, node *ast.Text) {
	switch {
	case node.HardLineBreak():
		builder.WriteString("\\\\\n")
	case node.SoftLineBreak():
		builder.WriteString("\n")
	}
}

// blockEnd is where the block holding node ends, which bounds how far a
// control sequence may reach when its groups straddle text nodes.
func (w *walker) blockEnd(node ast.Node) int {
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		// Lines panics on an inline node, and a text node inside emphasis has
		// inline parents before it reaches its block.
		if parent.Type() != ast.TypeBlock {
			continue
		}
		if lines := parent.Lines(); lines.Len() > 0 {
			return lines.At(lines.Len() - 1).Stop
		}
	}
	return len(w.source)
}

// emphasis renders emphasis and strong emphasis over their rendered content
// (srd002-renderer-core R4.2).
func (w *walker) emphasis(builder *strings.Builder, node *ast.Emphasis) error {
	content, err := w.inlineString(node)
	if err != nil {
		return err
	}
	command := `\emph`
	if node.Level >= 2 {
		command = `\textbf`
	}
	builder.WriteString(command + "{" + content + "}")
	return nil
}

// link renders a url command when the text equals the target and an href
// command otherwise (srd002-renderer-core R4.5).
func (w *walker) link(builder *strings.Builder, node *ast.Link) error {
	target := string(node.Destination)
	content, err := w.inlineString(node)
	if err != nil {
		return err
	}
	if content == latex.Escape(target) {
		builder.WriteString(`\url{` + target + `}`)
		return nil
	}
	builder.WriteString(`\href{` + target + `}{` + content + `}`)
	return nil
}

// citation validates the keys against the caller's set and renders the cite
// command (srd006-citations R2.1, R3.1).
func (w *walker) citation(builder *strings.Builder, node *cite.Node) error {
	if err := cite.Validate(node, w.config.Citations); err != nil {
		var unknown *cite.UnknownKeyError
		if ok := asUnknownKey(err, &unknown); ok {
			return w.fail(unknown.Offset, "citation", unknown.Error())
		}
		return err
	}
	return cite.Render(builder, node)
}

// rawHTML drops a comment and reports anything else
// (srd002-renderer-core R6.3).
func (w *walker) rawHTML(node *ast.RawHTML) error {
	if node.Segments.Len() > 0 {
		segment := node.Segments.At(0)
		if strings.HasPrefix(string(segment.Value(w.source)), "<!--") {
			return nil
		}
	}
	return w.fail(w.offsetOf(node), "raw HTML", "only comments are dropped; LaTeX is written as raw LaTeX")
}
