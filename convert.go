package mdtotex

import (
	"github.com/petar-djukic/md-to-tex/internal/render"
)

// Result is what one chapter conversion produces.
//
// The label report accompanies the fragment rather than replacing it: a
// caller needs both, and the labels of a fragment that was never produced
// describe nothing (srd-2-renderer-core R7.2).
type Result struct {
	// LaTeX is the chapter fragment. It carries no preamble and no document
	// environment, because the container inputs it (srd-2-renderer-core R1.4,
	// srd-8-container).
	LaTeX []byte
}

// Convert renders one chapter of Obsidian-native markdown as an IEEEtran
// LaTeX fragment.
//
// The name appears in error messages and nowhere else; Convert opens no file
// (srd-2-renderer-core R1.1). It holds no state between calls, so the same
// source and Options produce byte-identical output every time and concurrent
// calls do not interfere (R1.2).
//
// A construct outside the mapping - a table or a figure before those
// renderers land, a thematic break, raw HTML that is not a comment - returns
// an error naming the source, the line, and the construct, and no fragment
// (R1.3, R6.4).
func Convert(source []byte, name string, options Options) (Result, error) {
	fragment, _, err := render.Convert(source, name, render.Config{
		Citations: options.citations(),
	})
	if err != nil {
		return Result{}, err
	}
	return Result{LaTeX: fragment}, nil
}
