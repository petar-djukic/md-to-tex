package render

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// figure renders an image block as a float carrying its include, caption, and
// label (srd004-figures R2.1).
//
// The caption is the alt text and the label is the identifier the author
// stated, so both are written once in the markdown and neither is repeated in
// the LaTeX (srd004-figures R2.2, R2.3).
func (w *walker) figure(node *ast.Paragraph, image *ast.Image, attributes string) error {
	offset := w.offsetOf(node)

	parsed, err := parseAttributes(attributes)
	if err != nil {
		return w.fail(offset, "figure", err.Error())
	}
	if parsed.identifier == "" {
		return w.fail(offset, "figure",
			"states no identifier; a float nothing can reference belongs inline, so write it as {#fig:name}")
	}

	caption, err := w.inlineString(image)
	if err != nil {
		return err
	}
	if strings.TrimSpace(caption) == "" {
		return w.fail(offset, "figure",
			"has no alt text; the alt text is the caption, and LaTeX places a float away from the text that introduces it")
	}

	target := string(image.Destination)
	if err := checkTarget(target); err != nil {
		return w.fail(offset, "figure", err.Error())
	}

	environment, measure := "figure", `\columnwidth`
	if parsed.wide {
		environment, measure = "figure*", `\textwidth`
	}
	width := measure
	if parsed.width != "" {
		width = parsed.width + measure
	}

	w.out.WriteString(`\begin{` + environment + "}[!t]\n")
	w.out.WriteString("\\centering\n")
	w.out.WriteString(`\includegraphics[width=` + width + `]{` + path.Base(target) + "}\n")
	w.out.WriteString(`\caption{` + caption + "}\n")
	w.out.WriteString(`\label{` + parsed.identifier + "}\n")
	w.out.WriteString(`\end{` + environment + "}\n\n")
	return nil
}

// figureOf reports whether a paragraph is a figure: exactly one image, and
// nothing beside it but the attribute block that follows it
// (srd004-figures R1.1).
//
// Goldmark parses attributes on headings only, so the block arrives as prose
// (srd004-figures R1.2). It is read from the paragraph's source rather than
// from the nodes, because goldmark splits that prose at any character it might
// have parsed as emphasis: {#fig:Mixed_Case} arrives as two text nodes.
func figureOf(node *ast.Paragraph, source []byte) (*ast.Image, string, bool) {
	image, ok := node.FirstChild().(*ast.Image)
	if !ok {
		return nil, "", false
	}

	raw := strings.TrimSpace(blockText(node, source))
	if !strings.HasPrefix(raw, "![") {
		return nil, "", false
	}

	if !strings.HasSuffix(raw, "}") {
		// No attribute block. The float still needs an identifier, which R1.3
		// reports once the caller reaches the float path.
		return image, "", strings.HasSuffix(raw, ")")
	}

	open := strings.LastIndex(raw, "{")
	if open < 0 {
		return nil, "", false
	}
	// Everything between the image and its attribute block must be nothing at
	// all: an image with other content beside it is inline, not a float.
	if strings.TrimSpace(raw[:open]) == "" || !strings.HasSuffix(strings.TrimSpace(raw[:open]), ")") {
		return nil, "", false
	}
	return image, raw[open:], true
}

// blockText returns a block node's source lines, joined.
func blockText(node ast.Node, source []byte) string {
	var builder strings.Builder
	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		builder.Write(segment.Value(source))
	}
	return builder.String()
}

// attributes are what an image block's attribute block states.
type attributes struct {
	identifier string
	wide       bool
	width      string
}

// parseAttributes reads the identifier, the classes, and the key-value
// attributes an image block carries (srd004-figures R1.2, R3.1, R3.2, R3.3).
func parseAttributes(raw string) (attributes, error) {
	var parsed attributes
	if raw == "" {
		return parsed, nil
	}

	body := strings.TrimSuffix(strings.TrimPrefix(raw, "{"), "}")
	for _, field := range strings.Fields(body) {
		switch {
		case strings.HasPrefix(field, "#"):
			parsed.identifier = field[1:]
		case field == ".wide":
			parsed.wide = true
		case strings.HasPrefix(field, "."):
			return parsed, fmt.Errorf("carries the class %q, and the only class this mapping knows is .wide", field)
		case strings.HasPrefix(field, "width="):
			width, err := parseWidth(strings.TrimPrefix(field, "width="))
			if err != nil {
				return parsed, err
			}
			parsed.width = width
		default:
			return parsed, fmt.Errorf("carries the attribute %q, which this mapping does not know", field)
		}
	}
	return parsed, nil
}

// parseWidth reads a width attribute as a fraction of the enclosing measure.
// A fraction outside the range is an error naming the value, because a figure
// wider than its measure overruns the column silently (srd004-figures R3.3).
func parseWidth(value string) (string, error) {
	fraction, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return "", fmt.Errorf("states the width %q, which is not a fraction", value)
	}
	if fraction <= 0 || fraction > 1 {
		return "", fmt.Errorf("states the width %q; a width is a fraction of the measure, above zero and at most one", value)
	}
	return value, nil
}

// checkTarget rejects the targets a graphics path cannot resolve
// (srd004-figures R4.4).
func checkTarget(target string) error {
	switch {
	case target == "":
		return fmt.Errorf("names no file")
	case strings.Contains(target, "://"):
		return fmt.Errorf("names the remote target %q; the caller compiles figures locally", target)
	case strings.HasPrefix(target, "/"):
		return fmt.Errorf("names the absolute path %q; the container supplies the directory through its graphics path", target)
	}
	return nil
}
