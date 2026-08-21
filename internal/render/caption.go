package render

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// captionPrefix is what marks a paragraph as a table's caption line
// (srd005-tables R1.1).
const captionPrefix = "Table:"

// tableCaption is a caption line: the text, the identifier that becomes the
// float's label, and whether the author asked for the two-column float.
type tableCaption struct {
	text       string
	identifier string

	// wide is the class an author states when the measurement cannot see what
	// they know about the table (srd005-tables R4.6).
	wide bool
}

// captionFor reads the caption line that follows a table, and reports whether
// it consumed the following block (srd005-tables R1.1, R1.2, R1.3).
//
// A table with no caption line is an error: LaTeX places a float away from the
// text that introduces it, so an uncaptioned one is unidentifiable, and a
// table needing no caption is written as raw LaTeX in place
// (srd005-tables R1.4).
func (w *walker) captionFor(offset int, next ast.Node) (tableCaption, bool, error) {
	paragraph, ok := next.(*ast.Paragraph)
	if !ok {
		return tableCaption{}, false, w.fail(offset, "table",
			"has no caption line; write one as a paragraph beginning "+captionPrefix)
	}

	raw := strings.TrimSpace(blockText(paragraph, w.source))
	if !strings.HasPrefix(raw, captionPrefix) {
		return tableCaption{}, false, w.fail(offset, "table",
			"has no caption line; write one as a paragraph beginning "+captionPrefix)
	}

	body := strings.TrimSpace(strings.TrimPrefix(raw, captionPrefix))
	identifier, wide := "", false
	if strings.HasSuffix(body, "}") {
		if open := strings.LastIndex(body, "{"); open >= 0 {
			parsed, err := parseAttributes(body[open:])
			if err != nil {
				return tableCaption{}, false, w.fail(offset, "table", err.Error())
			}
			identifier, wide = parsed.identifier, parsed.wide
			body = strings.TrimSpace(body[:open])
		}
	}
	if identifier == "" {
		return tableCaption{}, false, w.fail(offset, "table",
			"states no identifier; a float nothing can reference belongs inline, so write the caption line as "+
				captionPrefix+" ... {#tab:name}")
	}

	rendered, err := w.inlineFragment(body, offset)
	if err != nil {
		return tableCaption{}, false, err
	}
	return tableCaption{text: rendered, identifier: identifier, wide: wide}, true, nil
}

// inlineFragment renders a run of markdown that is not a block of its own -- a
// caption line stripped of its prefix and attributes -- through the inline
// path, so escaping, emphasis, and citations behave as they do in a paragraph
// (srd005-tables R2.6, srd009-backport-compatibility R2.4).
func (w *walker) inlineFragment(fragment string, offset int) (string, error) {
	source := []byte(fragment)
	inner := &walker{source: source, name: w.name, config: w.config, seen: map[string]string{}}

	document := markdown.Parser().Parse(text.NewReader(source))
	paragraph := document.FirstChild()
	if paragraph == nil {
		return "", nil
	}

	rendered, err := inner.inlineString(paragraph)
	if err != nil {
		// The fragment's own positions mean nothing to a reader, so the
		// failure is reported where the table sits.
		var failure *Error
		if ok := asRenderError(err, &failure); ok {
			return "", w.fail(offset, failure.Construct, failure.Detail)
		}
		return "", err
	}
	return rendered, nil
}
