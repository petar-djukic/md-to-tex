package render

import (
	"bytes"

	"github.com/petar-djukic/md-to-tex/internal/frontmatter"
)

// dropFrontMatter removes a leading YAML frontmatter block from a chapter,
// keeping every line below it where it was (srd002-renderer-core R6.6).
//
// Obsidian writes frontmatter on chapters as a matter of course: it is where
// tags, aliases, and properties live, and the editor's own panel maintains
// them. None of it belongs in the LaTeX, and the opening fence would otherwise
// parse as a thematic break and be reported as an unmapped construct.
//
// The block's lines are kept as empty ones rather than removed, so a position
// in the body is the position in the file. Removing them outright would report
// an error at the twelfth line of a chapter as the fifth line of what was
// left, and send an author to the wrong place.
func dropFrontMatter(source []byte) []byte {
	_, body, offset, found := frontmatter.Split(source)
	if !found {
		return source
	}

	lines := bytes.Count(source[:offset], []byte{'\n'})
	padded := make([]byte, 0, lines+len(body))
	for i := 0; i < lines; i++ {
		padded = append(padded, '\n')
	}
	return append(padded, body...)
}
