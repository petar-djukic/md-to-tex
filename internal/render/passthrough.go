package render

import (
	"strings"

	"github.com/yuin/goldmark/ast"

	"github.com/petar-djukic/md-to-tex/internal/latex"
)

// writeText writes a run of markdown text, escaping the prose and passing the
// LaTeX an author wrote through untouched (srd-7-passthrough R2.1, R2.4).
//
// The split is what keeps the mapping bounded: a construct the library does
// not cover is written directly rather than added to the library
// (srd-2-renderer-core R6.5). It is also where the two failures meet - a
// backslash in ordinary prose must still escape (srd-7-passthrough R3.1), and
// a command must reach the output whole.
func (w *walker) writeText(builder *strings.Builder, content string, offset, blockEnd int) error {
	position := 0
	plainFrom := 0

	for position < len(content) {
		if content[position] != '\\' || !startsControlSequence(content[position:]) {
			position++
			continue
		}

		// The run is measured against the source rather than this text node,
		// because goldmark splits a text node at an opening bracket: the
		// optional argument of \sqrt[3]{27} arrives as three nodes. Measuring
		// per node would report every such command as unbalanced. The
		// lookahead stops at the end of the block, so an unclosed group is
		// still an error rather than swallowing the rest of the chapter.
		absolute := offset + position
		width, err := controlSequenceWidth(string(w.source[absolute:blockEnd]))
		if err != nil {
			return w.fail(absolute, "raw LaTeX", err.Error())
		}

		builder.WriteString(latex.Escape(content[plainFrom:position]))
		builder.WriteString(string(w.source[absolute : absolute+width]))

		if absolute+width > offset+len(content) {
			// The run continues past this node. Record how far it reached so
			// the nodes it covers are not written twice.
			w.rawUntil = absolute + width
			return nil
		}
		position += width
		plainFrom = position
	}

	builder.WriteString(latex.Escape(content[plainFrom:]))
	return nil
}

// startsControlSequence reports whether content opens a control sequence: a
// backslash followed by at least one ASCII letter. A backslash before anything
// else is prose and escapes (srd-7-passthrough R2.1, R3.1).
func startsControlSequence(content string) bool {
	return len(content) > 1 && content[0] == '\\' && isASCIILetter(content[1])
}

// controlSequenceWidth returns the byte width of the control sequence at the
// start of content: the command name and the balanced brace and bracket groups
// that follow it directly (srd-7-passthrough R2.1, R2.2).
//
// Recognition is textual and consults no list of known commands. An author who
// writes an undefined one gets a LaTeX error rather than a conversion error,
// which is the boundary srd-7-passthrough R3.5 draws.
func controlSequenceWidth(content string) (int, error) {
	position := 1
	for position < len(content) && isASCIILetter(content[position]) {
		position++
	}
	command := content[:position]

	for position < len(content) {
		opening := content[position]
		closing, isGroup := groupDelimiters(opening)
		if !isGroup {
			break
		}

		depth := 0
		groupStart := position
		for position < len(content) {
			switch content[position] {
			case opening:
				depth++
			case closing:
				depth--
			}
			position++
			if depth == 0 {
				break
			}
		}
		if depth != 0 {
			return 0, &unbalancedGroupError{command: command, opened: string(content[groupStart])}
		}
	}
	return position, nil
}

// groupDelimiters returns the closing delimiter for an argument group.
func groupDelimiters(opening byte) (byte, bool) {
	switch opening {
	case '{':
		return '}', true
	case '[':
		return ']', true
	}
	return 0, false
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// unbalancedGroupError is a control sequence whose argument group never
// closes. Emitting it would produce a LaTeX error pointing somewhere else
// entirely (srd-7-passthrough R2.3).
type unbalancedGroupError struct {
	command string
	opened  string
}

func (e *unbalancedGroupError) Error() string {
	return e.command + " opens " + e.opened + " and never closes it"
}

// fencedCode renders a code block as verbatim, or passes raw LaTeX through
// (srd-2-renderer-core R5.4, srd-7-passthrough R1.1, R1.2, R1.4).
//
// Raw block content is written as it stands: not parsed as markdown, not
// walked, and not searched for citations (srd-7-passthrough R1.5, R4.2).
func (w *walker) fencedCode(node *ast.FencedCodeBlock) error {
	body := codeText(node, w.source)
	if !isRawLaTeX(string(node.Language(w.source))) {
		return w.verbatim(body)
	}

	w.out.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		w.out.WriteString("\n")
	}
	w.out.WriteString("\n")
	return nil
}

// isRawLaTeX reports whether a fenced block's info string marks its content as
// raw LaTeX. Both markers appear in the manuscripts (srd-7-passthrough R1.3).
//
// Goldmark reports the info string with its braces, so ```{=latex} arrives as
// "{=latex}" rather than "=latex".
func isRawLaTeX(info string) bool {
	marker := strings.TrimSuffix(strings.TrimPrefix(info, "{"), "}")
	return marker == "=latex" || marker == "=tex"
}
