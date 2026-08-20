// Package container generates the document that inputs the chapter fragments.
//
// The container holds nothing worth editing -- class, preamble include,
// bibliography style, graphics path, the input list, the bibliography line --
// so regenerating it when a chapter joins the roster cannot disturb a chapter,
// and the preamble an author has adjusted survives because the container names
// that file rather than carrying its contents (srd008-container).
package container

import (
	"fmt"
	"path"
	"strings"
)

// Options is what the container needs from the caller. Every field has a
// default that matches the manuscripts, so a zero value generates the document
// they use (srd008-container R1.2, R1.5).
type Options struct {
	// DocumentClass is the class the paper compiles against. The manuscripts
	// use IEEEtran; the library neither ships nor validates a class file.
	DocumentClass string

	// ClassOptions are the options the class takes, as written between the
	// brackets.
	ClassOptions string

	// Preamble is the file the container includes by name. The author owns
	// its contents, and the library does not generate, read, or check it
	// (srd008-container R1.3).
	Preamble string

	// BibliographyStyle is the natbib style. The manuscripts use the IEEE one.
	BibliographyStyle string

	// Bibliography is the bibliography file, named without its extension. The
	// caller builds it from its reference corpus (srd008-container R1.5).
	Bibliography string

	// GraphicsPaths are the directories a figure included by base name is
	// searched for, relative to the directory the container compiles from
	// (srd008-container R1.4, srd004-figures R4.1).
	GraphicsPaths []string
}

// withDefaults fills what the caller left unset.
func (o Options) withDefaults() Options {
	if o.DocumentClass == "" {
		o.DocumentClass = "IEEEtran"
	}
	if o.ClassOptions == "" {
		o.ClassOptions = "conference"
	}
	if o.Preamble == "" {
		o.Preamble = "preamble"
	}
	if o.BibliographyStyle == "" {
		o.BibliographyStyle = "IEEEtranN"
	}
	if o.Bibliography == "" {
		o.Bibliography = "references"
	}
	if o.GraphicsPaths == nil {
		o.GraphicsPaths = []string{"../", "../fig/"}
	}
	return o
}

// Generate returns the container document for a roster of chapters
// (srd008-container R1.1).
//
// It writes nothing: whether to replace the container on disk is the caller's
// decision, and generation is deterministic so that decision can be made by
// comparing (srd008-container R3.1, R3.2).
func Generate(roster []string, options Options) ([]byte, error) {
	inputs, err := inputList(roster)
	if err != nil {
		return nil, err
	}
	options = options.withDefaults()

	var document strings.Builder
	fmt.Fprintf(&document, "\\documentclass[%s]{%s}\n", options.ClassOptions, options.DocumentClass)
	fmt.Fprintf(&document, "\\input{%s}\n", options.Preamble)
	fmt.Fprintf(&document, "\\bibliographystyle{%s}\n", options.BibliographyStyle)
	if len(options.GraphicsPaths) > 0 {
		document.WriteString(`\graphicspath{`)
		for _, directory := range options.GraphicsPaths {
			document.WriteString("{" + directory + "}")
		}
		document.WriteString("}\n")
	}
	document.WriteString("\\begin{document}\n")
	for _, input := range inputs {
		fmt.Fprintf(&document, "\\input{%s}\n", input)
	}
	fmt.Fprintf(&document, "\\bibliography{%s}\n", options.Bibliography)
	document.WriteString("\\end{document}\n")

	return []byte(document.String()), nil
}

// inputList maps the roster to input arguments, one to one and mechanically
// (srd008-container R2.1, R2.2, R2.3).
//
// The front matter is not special-cased: 00-front-matter.md becomes
// 00-front-matter like every other chapter.
func inputList(roster []string) ([]string, error) {
	if len(roster) == 0 {
		return nil, fmt.Errorf("the roster is empty; a document with no inputs compiles to an empty paper")
	}

	inputs := make([]string, 0, len(roster))
	seen := make(map[string]string, len(roster))
	for _, chapter := range roster {
		base := path.Base(chapter)
		// LaTeX supplies the extension, and naming it defeats the search path.
		name := strings.TrimSuffix(base, path.Ext(base))
		if name == "" {
			return nil, fmt.Errorf("the roster names %q, which has no chapter name", chapter)
		}
		if previous, clash := seen[name]; clash {
			return nil, fmt.Errorf(
				"the roster names %q and %q, whose fragments would collide as %s.tex",
				previous, chapter, name)
		}
		seen[name] = chapter
		inputs = append(inputs, name)
	}
	return inputs, nil
}
