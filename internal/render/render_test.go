package render

import (
	"strings"
	"testing"

	"github.com/petar-djukic/md-to-tex/internal/cite"
)

func convert(t *testing.T, source string) string {
	t.Helper()
	fragment, _, err := Convert([]byte(source), "chapter.md", Config{})
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}
	return string(fragment)
}

func convertError(t *testing.T, source string) *Error {
	t.Helper()
	_, _, err := Convert([]byte(source), "chapter.md", Config{})
	if err == nil {
		t.Fatalf("Convert() returned no error for:\n%s", source)
	}
	failure, ok := err.(*Error)
	if !ok {
		t.Fatalf("Convert() error = %T, want *Error", err)
	}
	return failure
}

// TestHeadingsRenderTheSRDExample covers srd002-renderer-core R3.1, R3.4, R3.5,
// and R3.7 with the example the SRD states.
func TestHeadingsRenderTheSRDExample(t *testing.T) {
	const source = "# The Conversion Pipeline {#sec:pipeline}\n\n## Escaping\n"
	const want = "\\section{The Conversion Pipeline}\\label{sec:pipeline}\n\n" +
		"\\subsection{Escaping}\\label{escaping}\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() =\n%q\nwant\n%q", got, want)
	}
}

// TestHeadingLevelsMapToSectioningCommands covers srd002-renderer-core R3.1.
func TestHeadingLevelsMapToSectioningCommands(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{"# One\n", `\section{One}\label{one}`},
		{"## Two\n", `\subsection{Two}\label{two}`},
		{"### Three\n", `\subsubsection{Three}\label{three}`},
		{"#### Four\n", `\paragraph{Four}\label{four}`},
	}

	for _, testCase := range cases {
		if got := strings.TrimSpace(convert(t, testCase.source)); got != testCase.want {
			t.Errorf("Convert(%q) = %q, want %q", testCase.source, got, testCase.want)
		}
	}
}

// TestDeepHeadingIsAnError covers srd002-renderer-core R3.2 and R1.3.
func TestDeepHeadingIsAnError(t *testing.T) {
	failure := convertError(t, "# One\n\n##### Five\n")

	if !strings.Contains(failure.Construct, "heading level 5") {
		t.Errorf("Construct = %q, want it to name the heading level", failure.Construct)
	}
	if failure.Line != 3 {
		t.Errorf("Line = %d, want 3", failure.Line)
	}
	if failure.Name != "chapter.md" {
		t.Errorf("Name = %q, want chapter.md", failure.Name)
	}
}

// TestDerivedSlugsMatchTheManuscriptReferences covers srd002-renderer-core R3.5
// against the identifiers the tutorial's chapters actually reference.
func TestDerivedSlugsMatchTheManuscriptReferences(t *testing.T) {
	cases := []struct {
		heading string
		want    string
	}{
		{"Literature survey", "literature-survey"},
		{"A governed agentic closed loop", "a-governed-agentic-closed-loop"},
		{"Conclusion", "conclusion"},
		{
			heading: "Use case 1: fault management with a declarative multi-agent system",
			want:    "use-case-1-fault-management-with-a-declarative-multi-agent-system",
		},
		{"Future directions", "future-directions"},
	}

	for _, testCase := range cases {
		got := strings.TrimSpace(convert(t, "# "+testCase.heading+"\n"))
		want := `\section{` + testCase.heading + `}\label{` + testCase.want + `}`
		if got != want {
			t.Errorf("Convert(%q) = %q, want %q", testCase.heading, got, want)
		}
	}
}

// TestCollidingHeadingsAreAnError covers srd002-renderer-core R3.6: both
// headings are named, rather than the second taking a counter suffix.
func TestCollidingHeadingsAreAnError(t *testing.T) {
	failure := convertError(t, "# Level mechanics\n\nProse.\n\n## Level mechanics\n")

	if !strings.Contains(failure.Construct, "level-mechanics") {
		t.Errorf("Construct = %q, want it to name the identifier", failure.Construct)
	}
	if strings.Count(failure.Detail, "Level mechanics") < 2 {
		t.Errorf("Detail = %q, want it to name both headings", failure.Detail)
	}
}

// TestStatedIdentifierResolvesACollision covers srd002-renderer-core R3.4: an
// author names one of the two and the chapter converts.
func TestStatedIdentifierResolvesACollision(t *testing.T) {
	const source = "# Level mechanics\n\nProse.\n\n## Level mechanics {#sec:level-mechanics-detail}\n"

	got := convert(t, source)
	if !strings.Contains(got, `\label{level-mechanics}`) {
		t.Errorf("first heading lost its derived label:\n%s", got)
	}
	if !strings.Contains(got, `\label{sec:level-mechanics-detail}`) {
		t.Errorf("second heading did not take its stated label:\n%s", got)
	}
}

// TestParagraphInlineRendering covers srd002-renderer-core R4.1, R4.2, and R4.3
// with the example the SRD states.
func TestParagraphInlineRendering(t *testing.T) {
	const source = "The *escaper* handles the ten `special` characters.\n"
	const want = "The \\emph{escaper} handles the ten \\texttt{special} characters.\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}
}

// TestStrongEmphasisRendersBold covers srd002-renderer-core R4.2.
func TestStrongEmphasisRendersBold(t *testing.T) {
	if got := convert(t, "A **bold** claim.\n"); got != "A \\textbf{bold} claim.\n" {
		t.Errorf("Convert() = %q", got)
	}
}

// TestTextReachesTheBufferThroughTheEscaper covers srd002-renderer-core R4.1
// and R6.2: a paragraph of specials comes out escaped, so no renderer writes
// text by another route.
func TestTextReachesTheBufferThroughTheEscaper(t *testing.T) {
	const source = "100% of R&D on item_one costs $5 {today}.\n"
	const want = "100\\% of R\\&D on item\\_one costs \\$5 \\{today\\}.\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}
}

// TestLineBreaks covers srd002-renderer-core R4.4: a soft break keeps the
// source's line structure, a hard break emits a LaTeX line break.
func TestLineBreaks(t *testing.T) {
	if got := convert(t, "first line\nsecond line\n"); got != "first line\nsecond line\n" {
		t.Errorf("soft break: Convert() = %q", got)
	}
	if got := convert(t, "first line  \nsecond line\n"); got != "first line\\\\\nsecond line\n" {
		t.Errorf("hard break: Convert() = %q", got)
	}
}

// TestLinks covers srd002-renderer-core R4.5.
func TestLinks(t *testing.T) {
	cases := []struct {
		source string
		want   string
	}{
		{
			source: "See [the survey](https://example.com/survey).\n",
			want:   "See \\href{https://example.com/survey}{the survey}.\n",
		},
		{
			source: "See [https://example.com](https://example.com).\n",
			want:   "See \\url{https://example.com}.\n",
		},
	}

	for _, testCase := range cases {
		if got := convert(t, testCase.source); got != testCase.want {
			t.Errorf("Convert(%q) = %q, want %q", testCase.source, got, testCase.want)
		}
	}
}

// TestListsAndQuotes covers srd002-renderer-core R5.1, R5.3, and R5.5 with the
// example the SRD states.
func TestListsAndQuotes(t *testing.T) {
	const source = "- forward conversion\n- backport\n"
	const want = "\\begin{itemize}\n\\item forward conversion\n\\item backport\n\\end{itemize}\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}

	ordered := convert(t, "1. first\n2. second\n")
	if !strings.HasPrefix(ordered, "\\begin{enumerate}\n\\item first\n") {
		t.Errorf("ordered list = %q", ordered)
	}

	quote := convert(t, "> A quotation.\n")
	if quote != "\\begin{quote}\nA quotation.\n\\end{quote}\n" {
		t.Errorf("blockquote = %q", quote)
	}
}

// TestNestedListsNest covers srd002-renderer-core R5.2.
func TestNestedListsNest(t *testing.T) {
	got := convert(t, "- outer\n    - inner\n")

	if strings.Count(got, `\begin{itemize}`) != 2 || strings.Count(got, `\end{itemize}`) != 2 {
		t.Errorf("nested list did not nest:\n%s", got)
	}
}

// TestCodeBlockRendersVerbatim covers srd002-renderer-core R5.4: a fenced block
// whose info string is not a raw-LaTeX marker is verbatim, unescaped.
func TestCodeBlockRendersVerbatim(t *testing.T) {
	const source = "```go\nif x > 0 && y_1 < 100% {\n}\n```\n"
	const want = "\\begin{verbatim}\nif x > 0 && y_1 < 100% {\n}\n\\end{verbatim}\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}
}

// TestRawLaTeXBlockPassesThrough covers srd002-renderer-core R5.4's dispatch to
// the passthrough path and srd007-passthrough R1.1, R1.2, and R1.3.
func TestRawLaTeXBlockPassesThrough(t *testing.T) {
	const source = "```{=latex}\n\\begin{IEEEkeywords}\nAutogenic systems, LLM agents.\n\\end{IEEEkeywords}\n```\n"
	const want = "\\begin{IEEEkeywords}\nAutogenic systems, LLM agents.\n\\end{IEEEkeywords}\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}

	if got := convert(t, "```{=tex}\n\\clearpage\n```\n"); got != "\\clearpage\n" {
		t.Errorf("=tex marker: Convert() = %q", got)
	}
}

// TestHTMLCommentsAreDropped covers srd002-renderer-core R6.3, which is what
// keeps the manuscripts' backlink comments out of the LaTeX.
func TestHTMLCommentsAreDropped(t *testing.T) {
	const source = "# Introduction\n\n<!-- S1 -- governed by docs/specs/software-requirements/srd001-front-matter.yaml -->\n\nProse.\n"

	got := convert(t, source)
	if strings.Contains(got, "governed by") || strings.Contains(got, "<!--") {
		t.Errorf("comment reached the fragment:\n%s", got)
	}
	if !strings.Contains(got, "Prose.") {
		t.Errorf("prose after the comment was lost:\n%s", got)
	}
}

// TestUnmappedConstructsAreErrors covers srd002-renderer-core R6.4 and R1.3:
// each names the construct and its line, and none is silently omitted. An
// image block is not among them: it renders as a float (srd004-figures).
func TestUnmappedConstructsAreErrors(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		construct string
		line      int
	}{
		{"thematic break", "Prose.\n\n---\n\nMore prose.\n", "thematic break", 3},
		{"raw HTML block", "Prose.\n\n<div>content</div>\n", "raw HTML", 3},
		{
			name:      "pipe table",
			source:    "Prose.\n\n| a | b |\n|---|---|\n| 1 | 2 |\n",
			construct: "table",
			line:      3,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			failure := convertError(t, testCase.source)
			if !strings.Contains(failure.Construct, testCase.construct) {
				t.Errorf("Construct = %q, want it to name %q", failure.Construct, testCase.construct)
			}
			if failure.Line != testCase.line {
				t.Errorf("Line = %d, want %d", failure.Line, testCase.line)
			}
		})
	}
}

// TestCitationsRenderThroughTheExtension covers srd002-renderer-core R6.1 and
// srd006-citations R2.3: the core dispatches to the citation renderer and does
// not duplicate its output.
func TestCitationsRenderThroughTheExtension(t *testing.T) {
	const source = "Converge [@du-2023; @alam-2024] and see [@zhang-2025b].\n"
	const want = "Converge \\cite{du-2023,alam-2024} and see \\cite{zhang-2025b}.\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}
}

// TestUnknownCitationKeyFailsTheConversion covers srd006-citations R3.1 and
// srd002-renderer-core R1.3: the error names the key and the line, and no
// fragment comes back with it.
func TestUnknownCitationKeyFailsTheConversion(t *testing.T) {
	const source = "Prose.\n\nConverge [@du-2023; @absent-key].\n"

	fragment, _, err := Convert([]byte(source), "chapter.md", Config{
		Citations: cite.NewKeySet("du-2023"),
	})
	if err == nil {
		t.Fatal("Convert() accepted an unknown key")
	}
	if fragment != nil {
		t.Errorf("Convert() returned a fragment alongside the error: %q", fragment)
	}

	failure, ok := err.(*Error)
	if !ok {
		t.Fatalf("error = %T, want *Error", err)
	}
	if !strings.Contains(failure.Detail, "absent-key") {
		t.Errorf("Detail = %q, want it to name the key", failure.Detail)
	}
	if failure.Line != 3 {
		t.Errorf("Line = %d, want 3", failure.Line)
	}
}

// TestFragmentShape covers srd002-renderer-core R1.4, R1.5, and AC11: the
// fragment ends with exactly one newline and does not begin with a blank line,
// whatever blank lines the source carried, and it carries no preamble.
func TestFragmentShape(t *testing.T) {
	got := convert(t, "\n\n# A heading\n\nProse.\n\n\n")

	if strings.HasPrefix(got, "\n") {
		t.Errorf("fragment begins with a blank line: %q", got)
	}
	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("fragment does not end with exactly one newline: %q", got)
	}
	for _, forbidden := range []string{`\documentclass`, `\begin{document}`, `\usepackage`} {
		if strings.Contains(got, forbidden) {
			t.Errorf("fragment carries %s", forbidden)
		}
	}
}

// TestConversionIsDeterministic covers srd002-renderer-core R1.2.
func TestConversionIsDeterministic(t *testing.T) {
	const source = "# A heading\n\nProse citing [@du-2023].\n\n- one\n- two\n"

	first := convert(t, source)
	for i := 0; i < 4; i++ {
		if got := convert(t, source); got != first {
			t.Fatalf("conversion %d differs:\n%q\nfrom\n%q", i, got, first)
		}
	}
}

// TestConcurrentConversionsDoNotInterfere covers srd002-renderer-core R1.2:
// conversion holds no state between calls. Run with -race.
func TestConcurrentConversionsDoNotInterfere(t *testing.T) {
	sources := []string{
		"# Alpha\n\nProse citing [@du-2023].\n",
		"# Beta\n\n- one\n- two\n",
		"# Gamma\n\n> A quotation.\n",
	}
	want := make([]string, len(sources))
	for i, source := range sources {
		want[i] = convert(t, source)
	}

	done := make(chan struct{})
	for i, source := range sources {
		go func(i int, source string) {
			defer func() { done <- struct{}{} }()
			for pass := 0; pass < 20; pass++ {
				fragment, _, err := Convert([]byte(source), "chapter.md", Config{})
				if err != nil {
					t.Errorf("Convert() error: %v", err)
					return
				}
				if string(fragment) != want[i] {
					t.Errorf("concurrent conversion %d differs: %q", i, fragment)
					return
				}
			}
		}(i, source)
	}
	for range sources {
		<-done
	}
}

// TestZeroConfigConverts covers srd002-renderer-core R2.3: a zero Config
// converts a chapter with no citations and no floats, and a citation renders
// because an absent key set turns validation off.
func TestZeroConfigConverts(t *testing.T) {
	fragment, _, err := Convert([]byte("# A heading\n\nProse citing [@anything].\n"), "chapter.md", Config{})
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}
	if !strings.Contains(string(fragment), `\cite{anything}`) {
		t.Errorf("citation did not render under a zero Config:\n%s", fragment)
	}
}

// TestHeadingTextTakesInlineConstructs covers srd002-renderer-core R3.3 and
// AC13: a heading carrying emphasis and LaTeX special characters renders them
// as a paragraph would, escaped and marked up.
func TestHeadingTextTakesInlineConstructs(t *testing.T) {
	const source = "# R&D on *guided* agents\n"
	const want = "\\section{R\\&D on \\emph{guided} agents}\\label{r-d-on-guided-agents}\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() = %q, want %q", got, want)
	}
}

// TestRawLaTeXIsTheRouteAroundTheMapping covers srd002-renderer-core R6.5 and
// srd007-passthrough R4.3: a construct the mapping does not cover is an error,
// and the same content written as raw LaTeX converts, so an author proceeds
// without waiting for a mapping.
func TestRawLaTeXIsTheRouteAroundTheMapping(t *testing.T) {
	failure := convertError(t, "Prose.\n\n---\n")
	if !strings.Contains(failure.Construct, "thematic break") {
		t.Fatalf("Construct = %q, want the thematic break reported", failure.Construct)
	}

	const written = "Prose.\n\n```{=latex}\n\\hrulefill\n```\n"
	got := convert(t, written)
	if !strings.Contains(got, `\hrulefill`) {
		t.Errorf("raw LaTeX did not reach the fragment:\n%s", got)
	}
}

// TestAMalformedCitationFailsTheConversion covers srd006-citations R4.2, R4.7,
// AC7, and srd002-renderer-core R6.4: a run that states the intent to cite and
// does not parse is an error naming the file and position, never literal text
// in a fragment.
//
// This is the failure the requirement was rewritten for. The run used to reach
// the fragment as brackets, read as prose, and typeset as brackets in the PDF,
// which is how eight citations left a converted paper with nothing reporting
// it.
func TestAMalformedCitationFailsTheConversion(t *testing.T) {
	cases := []struct {
		name   string
		source string
		line   int
	}{
		{"an unterminated run", "Prose.\n\nAn unterminated [@du-2023 run.\n", 3},
		{"an at sign with no key", "Prose.\n\nAn empty [@] run.\n", 3},
		{"a key carrying a space", "Prose.\n\nA spaced [@du 2023] run.\n", 3},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			failure := convertError(t, testCase.source)

			if failure.Construct != "citation" {
				t.Errorf("Construct = %q, want citation", failure.Construct)
			}
			if failure.Line != testCase.line {
				t.Errorf("Line = %d, want %d", failure.Line, testCase.line)
			}
			if !strings.Contains(failure.Detail, "does not parse as a citation") {
				t.Errorf("Detail = %q", failure.Detail)
			}
		})
	}
}

// TestNoFragmentCarriesLiteralCitationText covers srd006-citations AC7 over
// the whole mapping: whatever a chapter holds, a bracket-at run either becomes
// a cite command or fails the conversion.
func TestNoFragmentCarriesLiteralCitationText(t *testing.T) {
	sources := []string{
		"Cited [@du-2023].\n",
		"Cited [@du-2023; @alam-2024].\n",
		"# A heading citing [@du-2023]\n",
		"| A | B |\n|---|---|\n| [@du-2023] | 2 |\n\nTable: A caption. {#tab:c}\n",
		"![A caption citing [@du-2023].](fig/a.pdf){#fig:a}\n",
		"> A quotation citing [@du-2023].\n",
	}

	for _, source := range sources {
		fragment, _, err := Convert([]byte(source), "chapter.md", Config{})
		if err != nil {
			t.Errorf("Convert(%q) error: %v", source, err)
			continue
		}
		if strings.Contains(string(fragment), "[@") {
			t.Errorf("Convert(%q) left literal citation text:\n%s", source, fragment)
		}
	}
}

// TestTheKeysThatLeftThePaper covers srd006-citations R1.2 and AC8 with the
// evidence that found the defect: the eight keys a converted paper lost, and
// the group shape that lost them.
//
// Only one of the eight carries a non-ASCII letter. The other seven were
// ASCII, and were dropped because they shared a bracketed group with it.
func TestTheKeysThatLeftThePaper(t *testing.T) {
	lost := []string{
		"cheng-2009", "garlan2004rainbow", "lemos-2013", "mei-2024",
		"menasceé2011sassy", "rutten-2017", "sifakis-2025", "wu-2025",
	}

	source := "Self-adaptive systems are surveyed in [@" + strings.Join(lost, "; @") + "].\n"
	fragment, _, err := Convert([]byte(source), "02-literature-survey.md", Config{})
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}

	want := `\cite{` + strings.Join(lost, ",") + `}`
	if !strings.Contains(string(fragment), want) {
		t.Errorf("the group did not convert whole:\n got: %s\nwant: %s", fragment, want)
	}
	for _, key := range lost {
		if !strings.Contains(string(fragment), key) {
			t.Errorf("the fragment lost %q:\n%s", key, fragment)
		}
	}
}

// TestAWholeChapterConverts covers srd002-renderer-core AC1: a chapter of
// headings, paragraphs, emphasis, lists, quotations, and code converts to one
// fragment carrying no preamble of its own.
//
// The other tests take one construct each; this one takes them together, which
// is what the criterion asserts and what a chapter actually looks like.
func TestAWholeChapterConverts(t *testing.T) {
	const source = "# A heading\n\n" +
		"A paragraph with *emphasis*, **strength**, and `code`.\n\n" +
		"## A subheading\n\n" +
		"- a bullet\n- another\n\n" +
		"1. first\n2. second\n\n" +
		"> A quotation.\n\n" +
		"```go\nif x > 0 {\n}\n```\n"

	fragment := convert(t, source)

	for _, want := range []string{
		`\section{A heading}`,
		`\subsection{A subheading}`,
		`\emph{emphasis}`,
		`\textbf{strength}`,
		`\texttt{code}`,
		`\begin{itemize}`,
		`\begin{enumerate}`,
		`\begin{quote}`,
		`\begin{verbatim}`,
	} {
		if !strings.Contains(fragment, want) {
			t.Errorf("the chapter did not render %s:\n%s", want, fragment)
		}
	}
	for _, forbidden := range []string{`\documentclass`, `\begin{document}`, `\usepackage`} {
		if strings.Contains(fragment, forbidden) {
			t.Errorf("the fragment carries %s", forbidden)
		}
	}
}

// TestAChapterMayOpenWithFrontMatter covers srd002-renderer-core R6.6 and
// AC19: Obsidian writes frontmatter on chapters as a matter of course, so a
// chapter that opens with a block converts and its body renders exactly as the
// same body without one.
//
// This blocked every chapter of three papers: the opening fence parsed as a
// thematic break, which R6.4 correctly refused, and nothing read the block as
// metadata.
func TestAChapterMayOpenWithFrontMatter(t *testing.T) {
	const body = "# Introduction {#sec:intro}\n\nProse citing [@du-2023].\n\n- a bullet\n"

	cases := []struct {
		name  string
		block string
	}{
		{"tags, as the manuscripts write them", "---\ntags:\n  - autonomy-levels\n  - vision\n---\n\n"},
		{"a single scalar property", "---\nstatus: draft\n---\n\n"},
		{"an empty block", "---\n---\n\n"},
		{"properties Obsidian maintains", "---\naliases:\n  - intro\ncssclass: wide\n---\n\n"},
	}

	bare := convert(t, body)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := convert(t, testCase.block+body); got != bare {
				t.Errorf("the body did not render as it does without the block\n got:\n%s\nwant:\n%s", got, bare)
			}
		})
	}
}

// TestFrontMatterKeepsTheBodyWhereItIs covers srd002-renderer-core R6.8: a
// position reported in the body is the position in the file, so an author sent
// to a line finds the construct there.
func TestFrontMatterKeepsTheBodyWhereItIs(t *testing.T) {
	// The break sits on line 11 of the file: five lines of block, then a blank,
	// a heading, a blank, prose, a blank, and the break.
	const source = "---\ntags:\n  - a\n  - b\n---\n\n# Introduction\n\nProse.\n\n---\n"

	failure := convertError(t, source)
	if failure.Line != 11 {
		t.Errorf("Line = %d, want 11: the block's lines are kept so the body keeps its positions", failure.Line)
	}
}

// TestOnlyAnOpeningBlockIsFrontMatter covers srd002-renderer-core R6.7 and
// R6.4: a run of three hyphens below the opening is a thematic break, because
// a chapter carries at most one block and it is the first thing in the file.
func TestOnlyAnOpeningBlockIsFrontMatter(t *testing.T) {
	cases := []string{
		"# Introduction\n\nProse.\n\n---\n\nMore prose.\n",
		"---\ntags:\n  - a\n---\n\n# Introduction\n\n---\n\nProse.\n",
		"Prose before anything.\n\n---\ntags:\n  - a\n---\n",
	}

	for _, source := range cases {
		failure := convertError(t, source)
		if failure.Construct != "thematic break" {
			t.Errorf("Construct = %q for:\n%s", failure.Construct, source)
		}
	}
}

// TestAChapterWithoutFrontMatterIsUnchanged covers srd002-renderer-core R6.6:
// the drop reaches only a block that opens the source, so a chapter carrying
// none converts byte for byte as it did before the rule existed.
func TestAChapterWithoutFrontMatterIsUnchanged(t *testing.T) {
	const source = "# A heading\n\nA paragraph with *emphasis*.\n\n- one\n- two\n\n> A quotation.\n"
	const want = "\\section{A heading}\\label{a-heading}\n\n" +
		"A paragraph with \\emph{emphasis}.\n\n" +
		"\\begin{itemize}\n\\item one\n\\item two\n\\end{itemize}\n\n" +
		"\\begin{quote}\nA quotation.\n\\end{quote}\n"

	if got := convert(t, source); got != want {
		t.Errorf("Convert() =\n%q\nwant\n%q", got, want)
	}
}
