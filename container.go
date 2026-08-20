package mdtotex

import "github.com/petar-djukic/md-to-tex/internal/container"

// ContainerOptions is what the container document needs from the caller: the
// class and its options, the preamble file it includes by name, the
// bibliography and its style, and the graphics paths a figure resolves under.
//
// A zero value generates the document the manuscripts use.
type ContainerOptions = container.Options

// GenerateContainer returns the container document that inputs the chapter
// fragments, in roster order.
//
// It writes nothing. Generation is deterministic, so a caller compares the
// result against the container on disk to decide whether the roster changed,
// which is what keeps an author's preamble adjustments safe: the container
// names that file and never carries its contents
// (srd008-container R3.1, R3.2, R3.4).
func GenerateContainer(roster []string, options ContainerOptions) ([]byte, error) {
	return container.Generate(roster, options)
}
