package mdtotex

import (
	"github.com/petar-djukic/md-to-tex/internal/render"
)

// Result is what one chapter conversion produces.
//
// The label report accompanies the fragment rather than replacing it: a
// caller needs both, and the labels of a fragment that was never produced
// describe nothing (srd002-renderer-core R7.2).
type Result struct {
	// Name is the chapter the fragment came from, as it was passed to
	// Convert. Collisions reports it so a caller reading the collision knows
	// which chapters to look at.
	Name string

	// LaTeX is the chapter fragment. It carries no preamble and no document
	// environment, because the container inputs it (srd002-renderer-core R1.4,
	// srd008-container).
	LaTeX []byte

	// Labels are the identifiers the fragment carries, in the order the
	// chapter states them. Pass the results of several chapters to Collisions
	// to find identifiers more than one of them claims
	// (srd002-renderer-core R7.1).
	Labels []Label
}

// Convert renders one chapter of Obsidian-native markdown as an IEEEtran
// LaTeX fragment.
//
// The name appears in error messages and nowhere else; Convert opens no file
// (srd002-renderer-core R1.1). It holds no state between calls, so the same
// source and Options produce byte-identical output every time and concurrent
// calls do not interfere (R1.2).
//
// A construct outside the mapping - a table or a figure before those
// renderers land, a thematic break, raw HTML that is not a comment - returns
// an error naming the source, the line, and the construct, and no fragment
// (R1.3, R6.4).
func Convert(source []byte, name string, options Options) (Result, error) {
	fragment, labels, err := render.Convert(source, name, options.renderConfig())
	if err != nil {
		// A conversion that failed produced no fragment, so its labels
		// describe nothing (srd002-renderer-core R7.2).
		return Result{}, err
	}

	result := Result{Name: name, LaTeX: fragment}
	for _, label := range labels {
		result.Labels = append(result.Labels, Label{
			Identifier: label.Identifier,
			Heading:    label.Heading,
			Derived:    label.Derived,
		})
	}
	return result, nil
}
