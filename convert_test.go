package mdtotex_test

import (
	"strings"
	"testing"

	mdtotex "github.com/petar-djukic/md-to-tex"
)

// TestConvertRendersAChapter covers srd-2-renderer-core R1.1 and R1.4 across
// the public surface: source bytes and a name in, a fragment out, no file
// touched and no preamble emitted.
func TestConvertRendersAChapter(t *testing.T) {
	const source = "# Introduction {#sec:intro}\n\nProse citing [@du-2023].\n"

	result, err := mdtotex.Convert([]byte(source), "01-introduction.md", mdtotex.Options{})
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}

	fragment := string(result.LaTeX)
	want := "\\section{Introduction}\\label{sec:intro}\n\nProse citing \\cite{du-2023}.\n"
	if fragment != want {
		t.Errorf("Convert() =\n%q\nwant\n%q", fragment, want)
	}
}

// TestOptionsDistinguishAbsentFromEmptyKeys covers srd-2-renderer-core R2.3
// and srd-6-citations R3.2 through the public Options: nil turns validation
// off, an empty non-nil slice holds no valid key.
func TestOptionsDistinguishAbsentFromEmptyKeys(t *testing.T) {
	const source = "Prose citing [@du-2023].\n"

	if _, err := mdtotex.Convert([]byte(source), "chapter.md", mdtotex.Options{}); err != nil {
		t.Errorf("a nil key set should turn validation off, got: %v", err)
	}

	_, err := mdtotex.Convert([]byte(source), "chapter.md", mdtotex.Options{CitationKeys: []string{}})
	if err == nil {
		t.Error("an empty key set should reject every citation")
	}

	if _, err := mdtotex.Convert([]byte(source), "chapter.md",
		mdtotex.Options{CitationKeys: []string{"du-2023"}}); err != nil {
		t.Errorf("a key set holding the key should accept it, got: %v", err)
	}
}

// TestConvertReturnsNoFragmentWithAnError covers srd-2-renderer-core R1.3: a
// conversion never returns partial output alongside an error.
func TestConvertReturnsNoFragmentWithAnError(t *testing.T) {
	const source = "# A heading\n\nProse.\n\n---\n\nMore prose.\n"

	result, err := mdtotex.Convert([]byte(source), "chapter.md", mdtotex.Options{})
	if err == nil {
		t.Fatal("Convert() accepted a thematic break")
	}
	if result.LaTeX != nil {
		t.Errorf("Convert() returned %q alongside the error", result.LaTeX)
	}
	if !strings.Contains(err.Error(), "chapter.md:5") {
		t.Errorf("error = %q, want it to name the source and line", err)
	}
}

// TestConvertReadsNoConfiguration covers srd-2-renderer-core R2.1: Options is
// a plain value, so a caller constructs it in Go and the library reads no
// configuration file of its own.
func TestConvertReadsNoConfiguration(t *testing.T) {
	options := mdtotex.Options{CitationKeys: []string{"du-2023", "alam-2024"}}

	result, err := mdtotex.Convert([]byte("Cited [@alam-2024].\n"), "chapter.md", options)
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}
	if !strings.Contains(string(result.LaTeX), `\cite{alam-2024}`) {
		t.Errorf("Convert() = %q", result.LaTeX)
	}
}
