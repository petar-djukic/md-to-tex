package render

import (
	"strings"
	"testing"
)

// TestFigureRendersTheSRDExample covers srd004-figures R1.1, R1.2, R2.1, R2.2,
// R2.3, R4.1, and AC1 with the example the SRD states, byte for byte.
func TestFigureRendersTheSRDExample(t *testing.T) {
	const source = "![Autonomy levels graded by who writes the specification.](fig/01-kinds-to-levels.pdf){#fig:kinds-to-levels}\n"
	const want = "\\begin{figure}[!t]\n" +
		"\\centering\n" +
		"\\includegraphics[width=\\columnwidth]{01-kinds-to-levels.pdf}\n" +
		"\\caption{Autonomy levels graded by who writes the specification.}\n" +
		"\\label{fig:kinds-to-levels}\n" +
		"\\end{figure}\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() =\n%q\nwant\n%q", got, want)
	}
}

// TestWideFigureSpansBothColumns covers srd004-figures R3.1 and AC2 with the
// second example the SRD states.
func TestWideFigureSpansBothColumns(t *testing.T) {
	const source = "![The closed loop runs the algorithm its specification records.](fig/00-concept-structure.pdf){#fig:concept-structure .wide}\n"
	const want = "\\begin{figure*}[!t]\n" +
		"\\centering\n" +
		"\\includegraphics[width=\\textwidth]{00-concept-structure.pdf}\n" +
		"\\caption{The closed loop runs the algorithm its specification records.}\n" +
		"\\label{fig:concept-structure}\n" +
		"\\end{figure*}\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() =\n%q\nwant\n%q", got, want)
	}
}

// TestFigureWidthScalesTheEnclosingMeasure covers srd004-figures R3.2, R3.4,
// and AC2 with the third example, plus the two-column measure.
func TestFigureWidthScalesTheEnclosingMeasure(t *testing.T) {
	const source = "![A narrow diagram.](fig/02-narrow.pdf){#fig:narrow width=0.6}\n"
	const want = "\\begin{figure}[!t]\n" +
		"\\centering\n" +
		"\\includegraphics[width=0.6\\columnwidth]{02-narrow.pdf}\n" +
		"\\caption{A narrow diagram.}\n" +
		"\\label{fig:narrow}\n" +
		"\\end{figure}\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}

	wide := convert(t, "![Wide and narrowed.](fig/03-wide.pdf){#fig:wide .wide width=0.8}\n")
	if !strings.Contains(wide, `\includegraphics[width=0.8\textwidth]`) {
		t.Errorf("a wide figure scaled the wrong measure:\n%s", wide)
	}
}

// TestFigureKeepsItsFileName covers srd004-figures R4.1, R4.2, and AC3: the
// directory is dropped because the container supplies a graphics path, and the
// extension is never rewritten.
func TestFigureKeepsItsFileName(t *testing.T) {
	cases := []struct {
		target string
		want   string
	}{
		{"fig/01-kinds-to-levels.pdf", "01-kinds-to-levels.pdf"},
		{"figures/deep/02-nested.pdf", "02-nested.pdf"},
		{"03-beside-the-chapter.pdf", "03-beside-the-chapter.pdf"},
		{"fig/04-raster.png", "04-raster.png"},
	}

	for _, testCase := range cases {
		got := convert(t, "![A caption.]("+testCase.target+"){#fig:name}\n")
		if !strings.Contains(got, `\includegraphics[width=\columnwidth]{`+testCase.want+"}") {
			t.Errorf("target %q produced:\n%s", testCase.target, got)
		}
	}
}

// TestFigureCaptionTakesTheInlinePath covers srd004-figures R2.2: the caption
// escapes and renders inline constructs as a paragraph does.
func TestFigureCaptionTakesTheInlinePath(t *testing.T) {
	got := convert(t, "![R&D on *guided* agents, cited [@du-2023].](fig/05-agents.pdf){#fig:agents}\n")

	if !strings.Contains(got, `\caption{R\&D on \emph{guided} agents, cited \cite{du-2023}.}`) {
		t.Errorf("caption did not take the inline path:\n%s", got)
	}
}

// TestFigureIdentifierIsEmittedVerbatim covers srd004-figures R2.3:
// identifiers are not normalized, slugged, or prefixed.
func TestFigureIdentifierIsEmittedVerbatim(t *testing.T) {
	for _, identifier := range []string{"fig:kinds-to-levels", "fig:Mixed_Case.2", "diagram-1"} {
		got := convert(t, "![A caption.](fig/name.pdf){#"+identifier+"}\n")
		if !strings.Contains(got, `\label{`+identifier+"}") {
			t.Errorf("identifier %q was rewritten:\n%s", identifier, got)
		}
	}
}

// TestFigureErrors covers srd004-figures R1.3, R1.4, R3.3, R4.4, and AC4:
// each failure names the construct and its line rather than emitting a float
// nobody can reference.
func TestFigureErrors(t *testing.T) {
	cases := []struct {
		name   string
		source string
		detail string
	}{
		{
			name:   "no identifier",
			source: "Prose.\n\n![A caption.](fig/name.pdf)\n",
			detail: "states no identifier",
		},
		{
			name:   "no alt text",
			source: "Prose.\n\n![](fig/name.pdf){#fig:name}\n",
			detail: "has no alt text",
		},
		{
			name:   "width of zero",
			source: "Prose.\n\n![A caption.](fig/name.pdf){#fig:name width=0}\n",
			detail: "a width is a fraction of the measure",
		},
		{
			name:   "width above one",
			source: "Prose.\n\n![A caption.](fig/name.pdf){#fig:name width=1.5}\n",
			detail: "a width is a fraction of the measure",
		},
		{
			name:   "width that is not a number",
			source: "Prose.\n\n![A caption.](fig/name.pdf){#fig:name width=wide}\n",
			detail: "is not a fraction",
		},
		{
			name:   "remote target",
			source: "Prose.\n\n![A caption.](https://example.com/fig.pdf){#fig:name}\n",
			detail: "names the remote target",
		},
		{
			name:   "absolute target",
			source: "Prose.\n\n![A caption.](/var/figures/fig.pdf){#fig:name}\n",
			detail: "names the absolute path",
		},
		{
			name:   "a class the mapping does not know",
			source: "Prose.\n\n![A caption.](fig/name.pdf){#fig:name .rotated}\n",
			detail: "the only class this mapping knows is .wide",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			failure := convertError(t, testCase.source)
			if failure.Construct != "figure" {
				t.Errorf("Construct = %q, want figure", failure.Construct)
			}
			if !strings.Contains(failure.Detail, testCase.detail) {
				t.Errorf("Detail = %q, want it to name %q", failure.Detail, testCase.detail)
			}
			if failure.Line != 3 {
				t.Errorf("Line = %d, want 3", failure.Line)
			}
		})
	}
}

// TestInlineImageIsNotAFloat covers srd004-figures R1.1: an image with other
// content beside it is inline, and inline images have no mapping.
func TestInlineImageIsNotAFloat(t *testing.T) {
	failure := convertError(t, "Prose.\n\nSee ![A diagram.](fig/name.pdf){#fig:name} here.\n")

	if failure.Construct != "inline image" {
		t.Errorf("Construct = %q, want inline image", failure.Construct)
	}
	if !strings.Contains(failure.Detail, "paragraph of its own") {
		t.Errorf("Detail = %q", failure.Detail)
	}
}

// TestFigureReadsNoFile covers srd004-figures R4.3: the renderer does not
// check that the figure exists, because conversion opens no file.
func TestFigureReadsNoFile(t *testing.T) {
	got := convert(t, "![A caption.](fig/nothing-on-disk-anywhere.pdf){#fig:absent}\n")

	if !strings.Contains(got, "{nothing-on-disk-anywhere.pdf}") {
		t.Errorf("a figure that does not exist should still convert:\n%s", got)
	}
}

// TestFigureEmitsNoWrapper covers srd004-figures R2.4 and AC5: no adjustbox,
// and nothing between the float and its caption.
func TestFigureEmitsNoWrapper(t *testing.T) {
	got := convert(t, "![A caption.](fig/name.pdf){#fig:name .wide width=0.5}\n")

	for _, forbidden := range []string{"adjustbox", "refstepcounter", `\textbf{Figure`} {
		if strings.Contains(got, forbidden) {
			t.Errorf("emitted %s:\n%s", forbidden, got)
		}
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 6 || !strings.HasPrefix(lines[3], `\caption{`) || !strings.HasPrefix(lines[4], `\label{`) {
		t.Errorf("the float is not the six plain lines the SRD states:\n%s", got)
	}
}
