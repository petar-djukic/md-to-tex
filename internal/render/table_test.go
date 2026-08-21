package render

import (
	"strings"
	"testing"
)

const surveyTable = "| Survey | Scope | What this article adds |\n" +
	"|--------|-------|------------------------|\n" +
	"| Coronado et al., 2022 [@coronado-2022-ztn-survey] | Zero-touch automation for 5G/6G | Grades by who supplies the software |\n" +
	"| This article | Agentic AI across the autonomy levels | One frame: maturity, intent loop, governance |\n\n" +
	"Table: Adjacent surveys and what this article adds over each. {#tab:adjacent-surveys}\n"

const levelsTable = "| Level | Who writes it | Approval | Coverage | Example |\n" +
	"|-------|---------------|----------|----------|---------|\n" +
	"| L3 | Guided agent | Per change | Selected | Fault management |\n" +
	"| L4 | Guided agent | Pre-approved | Wide | Change management |\n\n" +
	"Table: Autonomy levels by authorship and approval. {#tab:levels}\n"

// TestTableRendersTheSRDExample covers srd005-tables R1.1, R1.2, R2.1, R2.2,
// R2.3, R2.5, R2.7, R3.6, and AC1 with the first example the SRD states, byte
// for byte -- including the column specification its weighting produces.
func TestTableRendersTheSRDExample(t *testing.T) {
	const want = "\\begin{table}[!t]\n" +
		"\\caption{Adjacent surveys and what this article adds over each.}\n" +
		"\\label{tab:adjacent-surveys}\n" +
		"\\centering\n" +
		"\\footnotesize\n" +
		"\\begin{tabularx}{\\columnwidth}{>{\\hsize=1.216\\hsize\\raggedright\\arraybackslash}X " +
		">{\\hsize=0.858\\hsize\\raggedright\\arraybackslash}X " +
		">{\\hsize=0.925\\hsize\\raggedright\\arraybackslash}X}\n" +
		"\\toprule\n" +
		"Survey & Scope & What this article adds \\\\\n" +
		"\\midrule\n" +
		"Coronado et al., 2022 \\cite{coronado-2022-ztn-survey} & Zero-touch automation for 5G/6G & Grades by who supplies the software \\\\\n" +
		"This article & Agentic AI across the autonomy levels & One frame: maturity, intent loop, governance \\\\\n" +
		"\\bottomrule\n" +
		"\\end{tabularx}\n" +
		"\\end{table}\n"

	if got := convert(t, surveyTable); got != want {
		t.Errorf("Convert() =\n%q\nwant\n%q", got, want)
	}
}

// TestWideTableRendersTheSRDExample covers srd005-tables R4.1, R4.5, and AC3
// with the second example: five columns select the starred float on the column
// count alone.
func TestWideTableRendersTheSRDExample(t *testing.T) {
	const want = "\\begin{table*}[!t]\n" +
		"\\caption{Autonomy levels by authorship and approval.}\n" +
		"\\label{tab:levels}\n" +
		"\\centering\n" +
		"\\footnotesize\n" +
		"\\begin{tabularx}{\\textwidth}{>{\\hsize=0.456\\hsize\\raggedright\\arraybackslash}X " +
		">{\\hsize=1.116\\hsize\\raggedright\\arraybackslash}X " +
		">{\\hsize=1.272\\hsize\\raggedright\\arraybackslash}X " +
		">{\\hsize=0.848\\hsize\\raggedright\\arraybackslash}X " +
		">{\\hsize=1.309\\hsize\\raggedright\\arraybackslash}X}\n" +
		"\\toprule\n" +
		"Level & Who writes it & Approval & Coverage & Example \\\\\n" +
		"\\midrule\n" +
		"L3 & Guided agent & Per change & Selected & Fault management \\\\\n" +
		"L4 & Guided agent & Pre-approved & Wide & Change management \\\\\n" +
		"\\bottomrule\n" +
		"\\end{tabularx}\n" +
		"\\end{table*}\n"

	if got := convert(t, levelsTable); got != want {
		t.Errorf("Convert() =\n%q\nwant\n%q", got, want)
	}
}

// TestColumnWeightsSumToTheColumnCount covers srd005-tables R3.1, R3.2, R3.3,
// R3.5, and AC2: whatever the damping, the floors, and the clamp do to a column,
// the weights are rescaled to sum to the column count, which is what tabularx
// requires of an hsize specification.
//
// The floors and the clamp are steps rather than invariants of the result: the
// rescale that restores the sum can carry a floored column back under its
// floor, which is what the reference filter does and what the SRD's own worked
// examples show.
func TestColumnWeightsSumToTheColumnCount(t *testing.T) {
	cases := []struct {
		name string
		body [][]string
	}{
		{"short, medium, and long", [][]string{
			{"L3", "Guided agent writes it", strings.Repeat("a long cell of prose ", 6)}}},
		{"one column carrying everything", [][]string{
			{"x", "y", strings.Repeat("z", 300)}}},
		{"an unbreakable word", [][]string{
			{"supercalifragilisticexpialidocious", "b", "c"}}},
		{"columns holding the same text", [][]string{
			{"alpha beta", "alpha beta", "alpha beta"}}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			weights := columnWeights(3, testCase.body, [][]string{{"A", "B", "C"}})
			if weights == nil {
				t.Fatal("columnWeights() returned nothing to weigh")
			}
			sum := 0.0
			for _, weight := range weights {
				if weight <= 0 {
					t.Errorf("weight %.3f is not a width", weight)
				}
				sum += weight
			}
			if sum < 2.999 || sum > 3.001 {
				t.Errorf("weights sum to %.3f, want the column count", sum)
			}
		})
	}
}

// TestLongerColumnsTakeMoreWidth covers srd005-tables R3.1 and R3.2: the
// weighting follows the cell contents, damped so ten times the text does not
// take ten times the measure.
func TestLongerColumnsTakeMoreWidth(t *testing.T) {
	weights := columnWeights(3, [][]string{
		{"a", "a medium cell here", strings.Repeat("a long cell of prose ", 8)},
	}, nil)

	if !(weights[0] < weights[1] && weights[1] < weights[2]) {
		t.Errorf("weights %v do not follow the cell lengths", weights)
	}
	if weights[2] > 10*weights[0] {
		t.Errorf("weights %v are undamped: the longest column took ten times the shortest", weights)
	}
}

// TestAnUnbreakableWordWidensItsColumn covers srd005-tables R3.4: a column is
// floored at the width its longest unbreakable word needs, because a narrower
// one overflows the rule. The same table with that word broken up weighs the
// column lower.
func TestAnUnbreakableWordWidensItsColumn(t *testing.T) {
	unbroken := columnWeights(3, [][]string{
		{"supercalifragilisticexpialidocious", "beta gamma delta", "epsilon zeta eta"},
	}, nil)
	broken := columnWeights(3, [][]string{
		{"super cali fragilistic expialidocious", "beta gamma delta", "epsilon zeta eta"},
	}, nil)

	if unbroken[0] <= broken[0] {
		t.Errorf("the unbreakable word did not widen its column: %v against %v", unbroken, broken)
	}
}

// TestControlSequencesDoNotCountAsWordLength covers srd005-tables R3.4: word
// length is measured after removing LaTeX control sequences and braces, so a
// citation does not widen a column by the length of its command.
func TestControlSequencesDoNotCountAsWordLength(t *testing.T) {
	cited := columnWeights(2, [][]string{{`\cite{du-2023}`, "beta"}}, nil)
	plain := columnWeights(2, [][]string{{"du-2023", "beta"}}, nil)

	if cited[0] < plain[0] {
		t.Errorf("the cite command counted against its column: %v against %v", cited, plain)
	}
}

// TestWideDecisionReadsMoreThanTheColumnCount covers srd005-tables R4.1, R4.2,
// R4.3, R4.4, and AC3: the width bar and the height bar each span a table the
// column count alone would leave in one column.
func TestWideDecisionReadsMoreThanTheColumnCount(t *testing.T) {
	long := strings.Repeat("x", 200)
	modest := []string{"alpha beta", "gamma delta"}

	cases := []struct {
		name    string
		columns int
		rows    [][]string
		want    bool
	}{
		{"a small three-column table", 3, [][]string{{"a", "b", "c"}, {"d", "e", "f"}}, false},
		{"five columns", 5, [][]string{{"a", "b", "c", "d", "e"}}, true},
		{"two columns of 200-character cells", 2, [][]string{{long, long}}, true},
		{"a long table of modest cells", 2, repeat(modest, 50), true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := spansBothColumns(testCase.columns, testCase.rows, testCase.rows)
			if got != testCase.want {
				t.Errorf("spansBothColumns() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func repeat(row []string, times int) [][]string {
	rows := make([][]string, times)
	for i := range rows {
		rows[i] = row
	}
	return rows
}

// TestTableCellsTakeTheInlinePath covers srd005-tables R2.6 and AC7: a cell
// escapes and renders citations and emphasis as a paragraph does.
func TestTableCellsTakeTheInlinePath(t *testing.T) {
	source := "| Claim | Source |\n|---|---|\n| R&D on *guided* agents | [@du-2023] |\n\n" +
		"Table: Claims and their sources. {#tab:claims}\n"

	got := convert(t, source)
	if !strings.Contains(got, `R\&D on \emph{guided} agents & \cite{du-2023}`) {
		t.Errorf("cells did not take the inline path:\n%s", got)
	}
}

// TestTableCaptionTakesTheInlinePath covers srd005-tables R2.1 and
// srd009-backport-compatibility R2.4: a caption carrying emphasis or a
// citation renders them.
func TestTableCaptionTakesTheInlinePath(t *testing.T) {
	source := "| A | B |\n|---|---|\n| 1 | 2 |\n\n" +
		"Table: R&D claims, *as surveyed* in [@du-2023]. {#tab:rd}\n"

	got := convert(t, source)
	if !strings.Contains(got, `\caption{R\&D claims, \emph{as surveyed} in \cite{du-2023}.}`) {
		t.Errorf("caption did not take the inline path:\n%s", got)
	}
}

// TestTableFontSizeIsConfigurable covers srd005-tables R2.7: the default is
// one step below the body, and a caller asking for body size gets no command.
func TestTableFontSizeIsConfigurable(t *testing.T) {
	if got := convert(t, surveyTable); !strings.Contains(got, "\\centering\n\\footnotesize\n") {
		t.Errorf("default size missing:\n%s", got)
	}

	empty := ""
	fragment, _, err := Convert([]byte(surveyTable), "chapter.md", Config{TableSize: &empty})
	if err != nil {
		t.Fatalf("Convert() error: %v", err)
	}
	if strings.Contains(string(fragment), `\footnotesize`) {
		t.Errorf("an empty size should emit no command:\n%s", fragment)
	}
	if !strings.Contains(string(fragment), "\\centering\n\\begin{tabularx}") {
		t.Errorf("the tabularx should follow the centering directly:\n%s", fragment)
	}
}

// TestTableErrors covers srd005-tables R1.3, R1.4, R1.5, R5.4, and AC4: each
// failure names the table and its line.
func TestTableErrors(t *testing.T) {
	cases := []struct {
		name   string
		source string
		detail string
	}{
		{
			name:   "no caption line",
			source: "Prose.\n\n| A | B |\n|---|---|\n| 1 | 2 |\n",
			detail: "has no caption line",
		},
		{
			name:   "a paragraph that is not a caption line",
			source: "Prose.\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\nOrdinary prose follows.\n",
			detail: "has no caption line",
		},
		{
			name:   "a caption line with no identifier",
			source: "Prose.\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\nTable: A caption with no identifier.\n",
			detail: "states no identifier",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			failure := convertError(t, testCase.source)
			if failure.Construct != "table" {
				t.Errorf("Construct = %q, want table", failure.Construct)
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

// TestCaptionParagraphElsewhereIsProse covers srd005-tables R1.3 and AC6: a
// paragraph opening with the caption prefix that follows no table is prose.
func TestCaptionParagraphElsewhereIsProse(t *testing.T) {
	got := convert(t, "Table: this sentence is about tables, not a caption.\n")

	if !strings.Contains(got, "Table: this sentence is about tables") {
		t.Errorf("the paragraph did not render as prose:\n%s", got)
	}
	if strings.Contains(got, `\caption`) {
		t.Errorf("a caption was emitted for a paragraph following no table:\n%s", got)
	}
}

// TestEmptyWeightingFallsBack covers srd005-tables R3.7 and AC8: a table with
// nothing to measure takes equal-width columns rather than failing.
func TestEmptyWeightingFallsBack(t *testing.T) {
	if got := columnSpec(3, nil, nil); got != "X X X" {
		t.Errorf("columnSpec() = %q, want equal-width columns", got)
	}
}

// TestTableEmitsNoForbiddenForm covers srd005-tables R2.4, R5.1, R5.2, R5.3,
// and AC5: no longtable, no strip, no bold header, and nothing between the
// float and its caption.
func TestTableEmitsNoForbiddenForm(t *testing.T) {
	for _, source := range []string{surveyTable, levelsTable} {
		got := convert(t, source)
		for _, forbidden := range []string{"longtable", "strip", "adjustbox", "refstepcounter", `\textbf{`} {
			if strings.Contains(got, forbidden) {
				t.Errorf("emitted %s:\n%s", forbidden, got)
			}
		}
		lines := strings.Split(strings.TrimSpace(got), "\n")
		if !strings.HasPrefix(lines[1], `\caption{`) || !strings.HasPrefix(lines[2], `\label{`) {
			t.Errorf("the caption and label do not open the float:\n%s", got)
		}
	}
}

// TestTheWideClassForcesTheStarredFloat covers srd005-tables R4.6, R4.8, and
// AC9: a caption line carrying the class spans both columns whatever the
// measurement would have chosen.
//
// The fixture is the tutorial table that found this gap: three columns, a
// longest cell of 99 characters, and an estimated height of 34 lines, so it
// falls under every bar R4.1 states and renders in one column without the
// class.
func TestTheWideClassForcesTheStarredFloat(t *testing.T) {
	const body = "| Survey or tutorial | Scope | What this article adds |\n" +
		"|---|---|---|\n" +
		"| Coronado et al., 2022 | Zero-touch automation for 5G and 6G networks | Grades by who supplies the software, not by platform capability |\n" +
		"| Leivadeas and Falkner, 2023 | Intent-based networking lifecycle | Makes intent the interface of a generative loop |\n" +
		"| Boateng et al., 2024 | LLMs across network and service management | Grades agent capability by autonomy level |\n\n"

	measured := convert(t, body+"Table: Adjacent surveys and what this article adds. {#tab:surveys}\n")
	if !strings.Contains(measured, `\begin{table}[!t]`) {
		t.Fatalf("the fixture no longer measures to one column, so it cannot show the class overriding:\n%s", measured)
	}

	stated := convert(t, body+"Table: Adjacent surveys and what this article adds. {#tab:surveys .wide}\n")
	if !strings.Contains(stated, `\begin{table*}[!t]`) {
		t.Errorf("the class did not force the starred float:\n%s", stated)
	}
	if !strings.Contains(stated, `\begin{tabularx}{\textwidth}`) {
		t.Errorf("the starred float is not at the text width:\n%s", stated)
	}
	if !strings.Contains(stated, `\end{table*}`) {
		t.Errorf("the float does not close as a starred one:\n%s", stated)
	}
}

// TestTheClassForcesSpanningAndNothingElse covers srd005-tables R4.7: without
// the class a table is measured exactly as before, and there is no class
// holding a table in one column that the measurement would widen.
func TestTheClassForcesSpanningAndNothingElse(t *testing.T) {
	// A table the measurement already widens keeps spanning, class or not.
	widened := convert(t, levelsTable)
	if !strings.Contains(widened, `\begin{table*}`) {
		t.Fatalf("the five-column fixture no longer spans:\n%s", widened)
	}

	// The class on an already-spanning table changes nothing.
	stated := strings.Replace(levelsTable, "{#tab:levels}", "{#tab:levels .wide}", 1)
	if got := convert(t, stated); got != widened {
		t.Errorf("the class changed a table that already spanned:\n got:\n%s\nwant:\n%s", got, widened)
	}
}

// TestNoExistingTableMoves covers srd005-tables R4.7 and AC9: a caption line
// without the class renders exactly what it rendered before the class existed.
//
// The two SRD fixtures are the check that matters, because their expected
// output is stated in the specification: if either moved, the class would have
// changed behavior nobody asked it to change.
func TestNoExistingTableMoves(t *testing.T) {
	survey := convert(t, surveyTable)
	if !strings.Contains(survey, `\begin{table}[!t]`) || strings.Contains(survey, `table*`) {
		t.Errorf("the three-column fixture moved:\n%s", survey)
	}
	if !strings.Contains(survey, `\begin{tabularx}{\columnwidth}`) {
		t.Errorf("the three-column fixture changed its measure:\n%s", survey)
	}

	levels := convert(t, levelsTable)
	if !strings.Contains(levels, `\begin{table*}[!t]`) {
		t.Errorf("the five-column fixture moved:\n%s", levels)
	}
}
