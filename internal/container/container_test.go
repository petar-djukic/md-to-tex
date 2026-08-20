package container

import (
	"strings"
	"testing"
)

// TestGenerateRendersTheSRDExample covers srd008-container R1.1, R1.2, R1.3,
// R1.4, R1.5, R2.1, R2.2, R2.3, and AC1 with the roster the SRD states, byte
// for byte.
func TestGenerateRendersTheSRDExample(t *testing.T) {
	roster := []string{"00-front-matter.md", "01-introduction.md", "02-literature-survey.md"}
	const want = "\\documentclass[conference]{IEEEtran}\n" +
		"\\input{preamble}\n" +
		"\\bibliographystyle{IEEEtranN}\n" +
		"\\graphicspath{{../}{../fig/}}\n" +
		"\\begin{document}\n" +
		"\\input{00-front-matter}\n" +
		"\\input{01-introduction}\n" +
		"\\input{02-literature-survey}\n" +
		"\\bibliography{references}\n" +
		"\\end{document}\n"

	got, err := Generate(roster, Options{})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if string(got) != want {
		t.Errorf("Generate() =\n%q\nwant\n%q", got, want)
	}
}

// TestFrontMatterIsNotSpecialCased covers srd008-container R2.2 and AC2: the
// front matter takes an input line on the same terms as every other chapter.
func TestFrontMatterIsNotSpecialCased(t *testing.T) {
	with, err := Generate([]string{"00-front-matter.md", "01-introduction.md"}, Options{})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	without, err := Generate([]string{"01-introduction.md"}, Options{})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.Contains(string(with), "\\input{00-front-matter}\n\\input{01-introduction}\n") {
		t.Errorf("the front matter is not an input like any other:\n%s", with)
	}
	if strings.Count(string(with), `\input{`)-strings.Count(string(without), `\input{`) != 1 {
		t.Error("adding the front matter changed more than one input line")
	}
}

// TestInputArgumentsCarryNoExtension covers srd008-container R2.1 and R2.3:
// the markdown extension is removed, no extension is added, and a directory
// prefix is dropped.
func TestInputArgumentsCarryNoExtension(t *testing.T) {
	got, err := Generate([]string{"chapters/01-introduction.md", "02-survey.md"}, Options{})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	document := string(got)
	for _, want := range []string{`\input{01-introduction}`, `\input{02-survey}`} {
		if !strings.Contains(document, want) {
			t.Errorf("Generate() = %q, want it to carry %q", document, want)
		}
	}
	for _, forbidden := range []string{".md", ".tex", "chapters/"} {
		if strings.Contains(document, forbidden) {
			t.Errorf("Generate() carries %q:\n%s", forbidden, document)
		}
	}
}

// TestRosterErrors covers srd008-container R2.4, R2.5, and AC3: an empty
// roster and a base-name collision each fail naming the problem.
func TestRosterErrors(t *testing.T) {
	cases := []struct {
		name   string
		roster []string
		detail string
	}{
		{
			name:   "an empty roster",
			roster: nil,
			detail: "the roster is empty",
		},
		{
			name:   "two chapters sharing a base name",
			roster: []string{"early/01-introduction.md", "late/01-introduction.md"},
			detail: "would collide as 01-introduction.tex",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Generate(testCase.roster, Options{})
			if err == nil {
				t.Fatal("Generate() accepted the roster")
			}
			if !strings.Contains(err.Error(), testCase.detail) {
				t.Errorf("error = %q, want it to name %q", err, testCase.detail)
			}
		})
	}
}

// TestGenerationIsDeterministic covers srd008-container R3.1, R3.2, and AC4:
// the same roster produces the same bytes, and nothing is written.
func TestGenerationIsDeterministic(t *testing.T) {
	roster := []string{"00-front-matter.md", "01-introduction.md"}

	first, err := Generate(roster, Options{})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	for i := 0; i < 4; i++ {
		again, err := Generate(roster, Options{})
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		if string(again) != string(first) {
			t.Fatalf("generation %d differs:\n%q\nfrom\n%q", i, again, first)
		}
	}
}

// TestOnlyTheInputListFollowsTheRoster covers srd008-container R3.3: nothing
// but the input list depends on the roster, so no chapter edit can require a
// regeneration.
func TestOnlyTheInputListFollowsTheRoster(t *testing.T) {
	before, err := Generate([]string{"01-introduction.md", "02-survey.md"}, Options{})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	after, err := Generate([]string{"01-introduction.md", "02-survey.md", "03-loop.md"}, Options{})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if difference := linesAdded(string(before), string(after)); difference != 1 {
		t.Errorf("a roster of one more chapter changed %d lines, want the one input line", difference)
	}
	if !strings.Contains(string(after), `\input{03-loop}`) {
		t.Errorf("the new chapter is not in the input list:\n%s", after)
	}
}

func linesAdded(before, after string) int {
	previous := map[string]int{}
	for _, line := range strings.Split(before, "\n") {
		previous[line]++
	}
	count := 0
	for _, line := range strings.Split(after, "\n") {
		if previous[line] > 0 {
			previous[line]--
			continue
		}
		count++
	}
	return count
}

// TestPreambleIsNamedNotCarried covers srd008-container R1.3, R3.4, and AC7:
// the container includes the preamble by reference, so an author's adjustment
// lives in a file the generator only names.
func TestPreambleIsNamedNotCarried(t *testing.T) {
	got, err := Generate([]string{"01-introduction.md"}, Options{Preamble: "house-style"})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	document := string(got)
	if !strings.Contains(document, `\input{house-style}`) {
		t.Errorf("the preamble is not included by name:\n%s", document)
	}
	if strings.Contains(document, `\usepackage`) || strings.Contains(document, `\newcommand`) {
		t.Errorf("the container carries preamble content:\n%s", document)
	}
}

// TestOptionsCarryTheContainerFields covers srd008-container R1.2 and R1.5,
// and srd002-renderer-core R2.2, which leaves these fields to this SRD.
func TestOptionsCarryTheContainerFields(t *testing.T) {
	got, err := Generate([]string{"01-introduction.md"}, Options{
		DocumentClass:     "IEEEtranN",
		ClassOptions:      "journal,twocolumn",
		BibliographyStyle: "plainnat",
		Bibliography:      "corpus",
		GraphicsPaths:     []string{"../figures/"},
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	document := string(got)
	for _, want := range []string{
		`\documentclass[journal,twocolumn]{IEEEtranN}`,
		`\bibliographystyle{plainnat}`,
		`\bibliography{corpus}`,
		`\graphicspath{{../figures/}}`,
	} {
		if !strings.Contains(document, want) {
			t.Errorf("Generate() = %q, want it to carry %q", document, want)
		}
	}
}

// TestContainerHoldsNoContent covers srd008-container R1.6 and AC5: every line
// is a declaration, an input, or an environment delimiter.
func TestContainerHoldsNoContent(t *testing.T) {
	got, err := Generate([]string{"00-front-matter.md", "01-introduction.md"}, Options{})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(got)), "\n") {
		if !strings.HasPrefix(line, `\`) {
			t.Errorf("the container carries a line that is not a command: %q", line)
		}
	}
	for _, forbidden := range []string{`\section`, `\begin{figure`, `\begin{table`, `\caption`} {
		if strings.Contains(string(got), forbidden) {
			t.Errorf("the container carries %s", forbidden)
		}
	}
}

// TestGraphicsPathResolvesAFigure covers srd008-container R1.4 and AC6, with
// srd004-figures R4.1: a figure included by base name is searched for in the
// directories the container names.
func TestGraphicsPathResolvesAFigure(t *testing.T) {
	got, err := Generate([]string{"01-introduction.md"}, Options{})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	document := string(got)
	if !strings.Contains(document, `\graphicspath{{../}{../fig/}}`) {
		t.Errorf("the graphics path does not name the figure directory:\n%s", document)
	}
	if strings.Index(document, `\graphicspath`) > strings.Index(document, `\begin{document}`) {
		t.Errorf("the graphics path is emitted after the document opens:\n%s", document)
	}
}
