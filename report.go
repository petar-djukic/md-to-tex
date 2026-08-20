package mdtotex

// Label is one identifier a converted chapter carries, with the heading it
// came from (srd-2-renderer-core R7.1).
//
// A caller cannot recover this from the fragment without parsing LaTeX, and
// the renderer has it already, so conversion reports it rather than making
// every caller find it again.
type Label struct {
	// Identifier is what the label command carries, which is what a
	// cross-reference names.
	Identifier string

	// Heading is the text the label belongs to, so a report about two
	// chapters can say which headings collided.
	Heading string

	// Derived says whether the identifier came from the heading text rather
	// than from an identifier the author stated. Both kinds collide the same
	// way (srd-2-renderer-core R7.5); this records which is which for a
	// caller deciding what to tell the author.
	Derived bool
}
