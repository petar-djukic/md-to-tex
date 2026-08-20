// Package mdtotex converts Obsidian-native markdown manuscripts into IEEEtran
// LaTeX.
//
// Convert renders one chapter as a fragment the container document inputs.
// Markdown is the source of truth: chapter and figure file names match
// one-to-one across the two formats, a caption is written once and reaches
// the LaTeX as a plain caption command, and the emitted forms stay close
// enough to what an author hand-writes that an edit on the LaTeX side comes
// back through the pipeline's backport as prose.
//
// The specifications under docs/ govern the conversion. Each component names
// the SRD it answers to, and docs/ARCHITECTURE.yaml maps the two.
package mdtotex
