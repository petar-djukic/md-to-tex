package render

import (
	"strings"
	"testing"
)

// TestInlineControlSequencesPassThrough covers srd007-passthrough R2.1, R2.4,
// and AC2 with the example the SRD states: the commands survive, the prose
// around them escapes.
func TestInlineControlSequencesPassThrough(t *testing.T) {
	const source = "Section \\ref{literature-survey} surveys the standards, and Figure \\ref{fig:kinds-to-levels} grades them.\n"
	const want = "Section \\ref{literature-survey} surveys the standards, and Figure \\ref{fig:kinds-to-levels} grades them.\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}
}

// TestRawAndEscapedTextShareAParagraph covers srd007-passthrough R2.4, R3.1,
// R4.1, srd003-escaping R3.4, and AC2 together: raw content is written to the
// buffer directly and never reaches the escaper, while the prose beside it
// escapes.
func TestRawAndEscapedTextShareAParagraph(t *testing.T) {
	const source = "R&D on \\emph{guided} agents, 100% of it.\n"
	const want = "R\\&D on \\emph{guided} agents, 100\\% of it.\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}
}

// TestNestedGroupsSurviveWhole covers srd007-passthrough R2.2: a group closes
// at the brace that balances its opening, so an argument holding braces is not
// cut short.
func TestNestedGroupsSurviveWhole(t *testing.T) {
	cases := []string{
		"A \\textbf{bold \\emph{and italic}} run.\n",
		"A \\footnote{see \\ref{sec:intro} for the detail} note.\n",
		"An optional \\sqrt[3]{27} argument.\n",
		"Both \\rule[2pt]{1em}{1pt} groups.\n",
	}

	for _, source := range cases {
		if got := convert(t, source); got != source {
			t.Errorf("Convert(%q) = %q, want it unchanged", source, got)
		}
	}
}

// TestUnbalancedGroupIsAnError covers srd007-passthrough R2.3 and AC3: the
// error names the command and the line rather than letting LaTeX report it
// somewhere else entirely.
func TestUnbalancedGroupIsAnError(t *testing.T) {
	failure := convertError(t, "Prose.\n\nA \\ref{sec:intro command that never closes.\n")

	if !strings.Contains(failure.Detail, `\ref`) {
		t.Errorf("Detail = %q, want it to name the command", failure.Detail)
	}
	if failure.Line != 3 {
		t.Errorf("Line = %d, want 3", failure.Line)
	}
}

// TestBackslashInProseEscapes covers srd007-passthrough R3.1 and AC5: a
// backslash before anything but an ASCII letter is prose.
func TestBackslashInProseEscapes(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{"A backslash \\ in prose.\n", "A backslash \\textbackslash{} in prose.\n"},
		{"A backslash \\2026 before a digit.\n", "A backslash \\textbackslash{}2026 before a digit.\n"},
		{"A backslash \\- before punctuation.\n", "A backslash \\textbackslash{}- before punctuation.\n"},
	}

	for _, testCase := range cases {
		if got := convert(t, testCase.source); got != testCase.want {
			t.Errorf("Convert(%q) = %q, want %q", testCase.source, got, testCase.want)
		}
	}
}

// TestBackslashInCodeIsCode covers srd007-passthrough R3.2: a backslash inside
// inline code or a code block is neither raw nor escaped by this path.
func TestBackslashInCodeIsCode(t *testing.T) {
	got := convert(t, "The command `\\ref{x}` in prose.\n")
	if !strings.Contains(got, `\texttt{`) {
		t.Errorf("inline code did not render as monospace: %q", got)
	}
	if strings.Contains(got, `\texttt{\ref{x}}`) {
		t.Errorf("code content passed through raw rather than escaping: %q", got)
	}

	block := convert(t, "```\n\\ref{x}\n```\n")
	if !strings.Contains(block, "\\begin{verbatim}\n\\ref{x}\n") {
		t.Errorf("code block did not render verbatim: %q", block)
	}
}

// TestRawBlockContentIsNotWalked covers srd007-passthrough R1.5, R3.3, R4.2,
// srd006-citations R4.4, and AC4: a citation inside a raw block stays as
// written, and an image inside one does not become a float.
func TestRawBlockContentIsNotWalked(t *testing.T) {
	const source = "```{=latex}\n\\cite{du-2023} and ![A diagram](fig/diagram.pdf)\n```\n"

	got := convert(t, source)
	if got != "\\cite{du-2023} and ![A diagram](fig/diagram.pdf)\n" {
		t.Errorf("Convert() = %q, want the block byte for byte", got)
	}
}

// TestRawBlockKeepsItsInfoStringOut covers srd007-passthrough R1.1, R1.2, and
// AC1: the content reaches the fragment unescaped, without the fence.
func TestRawBlockKeepsItsInfoStringOut(t *testing.T) {
	const source = "```{=latex}\n\\begin{IEEEkeywords}\nAutogenic systems, 100% agents.\n\\end{IEEEkeywords}\n```\n"
	const want = "\\begin{IEEEkeywords}\nAutogenic systems, 100% agents.\n\\end{IEEEkeywords}\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}
	if strings.Contains(convert(t, source), "=latex") {
		t.Error("the info string reached the fragment")
	}
}

// TestOtherInfoStringsAreVerbatim covers srd007-passthrough R1.4 and AC6: the
// distinction is the info string alone.
func TestOtherInfoStringsAreVerbatim(t *testing.T) {
	got := convert(t, "```go\n\\ref{x}\n```\n")

	if !strings.HasPrefix(got, "\\begin{verbatim}") {
		t.Errorf("Convert() = %q, want a verbatim environment", got)
	}
}

// TestUnknownCommandsAreNotChecked covers srd007-passthrough R3.5: recognition
// is textual, so a command this library never heard of passes through and
// LaTeX is left to report it.
func TestUnknownCommandsAreNotChecked(t *testing.T) {
	const source = "A \\notacommand{argument} here.\n"

	if got := convert(t, source); got != source {
		t.Errorf("Convert() = %q, want it unchanged", got)
	}
}

// TestEnvironmentsWrittenInProseConvertAround covers srd007-passthrough
// non-goals: a begin command and its end are two control sequences with prose
// between them, and the prose converts normally.
func TestEnvironmentsWrittenInProseConvertAround(t *testing.T) {
	const source = "\\begin{center}\n\nR&D prose inside.\n\n\\end{center}\n"

	got := convert(t, source)
	if !strings.Contains(got, `\begin{center}`) || !strings.Contains(got, `\end{center}`) {
		t.Errorf("the environment commands did not survive: %q", got)
	}
	if !strings.Contains(got, `R\&D prose inside.`) {
		t.Errorf("the prose between them did not convert: %q", got)
	}
}

// TestHardBreakIsNotAControlSequence covers srd007-passthrough R3.4.
func TestHardBreakIsNotAControlSequence(t *testing.T) {
	if got := convert(t, "first line  \nsecond line\n"); got != "first line\\\\\nsecond line\n" {
		t.Errorf("Convert() = %q", got)
	}
}

// TestRawLaTeXOwnsWhatItCarries covers srd007-passthrough R4.4: raw LaTeX
// carrying a float and a caption is not held to the float requirements, and
// reaches the fragment as the author wrote it.
func TestRawLaTeXOwnsWhatItCarries(t *testing.T) {
	const source = "```{=latex}\n" +
		"\\begin{figure}[!t]\n\\centering\n\\includegraphics{01-diagram.pdf}\n" +
		"\\caption{A caption the author wrote.}\n\\label{fig:diagram}\n\\end{figure}\n" +
		"```\n"

	got := convert(t, source)

	want := "\\begin{figure}[!t]\n\\centering\n\\includegraphics{01-diagram.pdf}\n" +
		"\\caption{A caption the author wrote.}\n\\label{fig:diagram}\n\\end{figure}\n"
	if got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}
}
