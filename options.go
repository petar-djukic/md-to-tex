package mdtotex

import "github.com/petar-djukic/md-to-tex/internal/cite"

// Options is what a caller configures for a conversion.
//
// It is a plain value holding no file paths to read and no callbacks: the
// caller may keep it in its own configuration file, and this library reads
// none (srd-2-renderer-core R2.1). The zero value converts a chapter that
// carries no citations (R2.3).
type Options struct {
	// CitationKeys is the set of keys a citation may name, which the caller
	// takes from its reference corpus.
	//
	// A nil slice turns key validation off. A non-nil empty slice holds no
	// valid key, so every citation fails against it. The two are
	// distinguishable, which is what srd-6-citations R3.2 requires.
	CitationKeys []string
}

// citations converts the caller's keys into the set the renderer validates
// against, preserving the difference between absent and empty.
func (o Options) citations() cite.KeySet {
	if o.CitationKeys == nil {
		return nil
	}
	return cite.NewKeySet(o.CitationKeys...)
}
