package mdtotex_test

import (
	"strings"
	"testing"

	mdtotex "github.com/petar-djukic/md-to-tex"
)

// TestGenerateContainerAssemblesTheDocument covers srd008-container R1.1,
// R2.1, and R3.1 across the public surface: a roster in, a document out,
// nothing written.
func TestGenerateContainerAssemblesTheDocument(t *testing.T) {
	roster := []string{"00-front-matter.md", "01-introduction.md"}

	document, err := mdtotex.GenerateContainer(roster, mdtotex.ContainerOptions{})
	if err != nil {
		t.Fatalf("GenerateContainer() error: %v", err)
	}

	got := string(document)
	if !strings.HasPrefix(got, `\documentclass[conference]{IEEEtran}`) {
		t.Errorf("GenerateContainer() = %q", got)
	}
	if !strings.HasSuffix(got, "\\end{document}\n") {
		t.Errorf("GenerateContainer() = %q", got)
	}
}

// TestTheDocumentShellFitsTogether covers srd001-front-matter R4.5 and
// srd008-container R2.2: the front-matter fragment carries no preamble, and
// the container inputs it by the name its chapter file states.
func TestTheDocumentShellFitsTogether(t *testing.T) {
	const titlePage = "---\ntitle: A Paper\nauthor: Petar Djukic\nabstract: An abstract.\n---\n"

	front, err := mdtotex.RenderFrontMatter([]byte(titlePage), "00-front-matter.md", mdtotex.Options{})
	if err != nil {
		t.Fatalf("RenderFrontMatter() error: %v", err)
	}
	chapter, err := mdtotex.Convert([]byte("# Introduction\n\nProse.\n"), "01-introduction.md", mdtotex.Options{})
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}
	document, err := mdtotex.GenerateContainer(
		[]string{front.Name, chapter.Name}, mdtotex.ContainerOptions{})
	if err != nil {
		t.Fatalf("GenerateContainer() error: %v", err)
	}

	for _, fragment := range []string{string(front.LaTeX), string(chapter.LaTeX)} {
		for _, forbidden := range []string{`\documentclass`, `\begin{document}`, `\bibliography`} {
			if strings.Contains(fragment, forbidden) {
				t.Errorf("a fragment carries %s, which belongs to the container:\n%s", forbidden, fragment)
			}
		}
	}
	for _, want := range []string{`\input{00-front-matter}`, `\input{01-introduction}`} {
		if !strings.Contains(string(document), want) {
			t.Errorf("the container does not input the fragment: %q", want)
		}
	}
}
