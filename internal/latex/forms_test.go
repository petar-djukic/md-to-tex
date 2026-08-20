package latex

import (
	"strings"
	"testing"
)

// TestForbiddenFindsEachForm covers srd009-backport-compatibility R3.1, R3.2,
// R3.3, R3.4, R3.5, and AC3: every form the SRD forbids is found and named,
// with the line it sits on.
func TestForbiddenFindsEachForm(t *testing.T) {
	cases := []struct {
		name  string
		latex string
		form  string
	}{
		{
			name:  "adjustbox around a float's content",
			latex: "\\begin{figure}[!t]\n\\begin{adjustbox}{max width=\\columnwidth}\n",
			form:  "adjustbox",
		},
		{
			name:  "a hand-assembled caption",
			latex: "\\begin{table}[!t]\n\\refstepcounter{table}\n\\noindent\\textbf{Table \\thetable.} A caption\n",
			form:  "refstepcounter",
		},
		{
			name:  "a bold header row",
			latex: "\\toprule\n\\textbf{Survey} & \\textbf{Scope} \\\\\n\\midrule\n",
			form:  "a bold header row",
		},
		{
			name:  "a strip environment",
			latex: "\\begin{strip}\n\\centering\n",
			form:  "strip",
		},
		{
			name:  "a longtable",
			latex: "\\begin{longtable}{ll}\n",
			form:  "longtable",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			found := Forbidden(testCase.latex)
			if len(found) == 0 {
				t.Fatalf("Forbidden() found nothing in:\n%s", testCase.latex)
			}
			if found[0].Form != testCase.form {
				t.Errorf("Form = %q, want %q", found[0].Form, testCase.form)
			}
			if found[0].Line < 1 {
				t.Errorf("Line = %d, want the line it sits on", found[0].Line)
			}
			if found[0].Why == "" {
				t.Error("a finding should say what the form breaks")
			}
		})
	}
}

// TestForbiddenPassesThePermittedForms covers srd009-backport-compatibility
// R2.1 and R2.2: the plain float the SRD permits carries nothing forbidden.
func TestForbiddenPassesThePermittedForms(t *testing.T) {
	const permitted = "\\begin{figure}[!t]\n" +
		"\\centering\n" +
		"\\includegraphics[width=\\columnwidth]{01-kinds-to-levels.pdf}\n" +
		"\\caption{Autonomy levels graded by who writes the specification.}\n" +
		"\\label{fig:kinds-to-levels}\n" +
		"\\end{figure}\n"

	if found := Forbidden(permitted); len(found) != 0 {
		t.Errorf("Forbidden() flagged the permitted form: %v", found)
	}
	if found := FloatsCarryPlainCaptions(permitted); len(found) != 0 {
		t.Errorf("FloatsCarryPlainCaptions() flagged the permitted form: %v", found)
	}
}

// TestBoldCellsOutsideAHeaderAreNotFlagged covers
// srd009-backport-compatibility R3.3: the rule is about a header row of bold
// cells, not about emphasis an author wrote inside a sentence.
func TestBoldCellsOutsideAHeaderAreNotFlagged(t *testing.T) {
	const prose = "A sentence with \\textbf{emphasis} an author wrote.\n"

	if found := Forbidden(prose); len(found) != 0 {
		t.Errorf("Forbidden() flagged an author's own emphasis: %v", found)
	}
}

// TestFloatsCarryPlainCaptions covers srd009-backport-compatibility R2.1, R2.2,
// and R2.3: a float needs the caption and label pair, in that order, for both
// float kinds.
func TestFloatsCarryPlainCaptions(t *testing.T) {
	cases := []struct {
		name  string
		latex string
		want  string
	}{
		{
			name: "a table captioned before its body",
			latex: "\\begin{table}[!t]\n\\caption{A caption.}\n\\label{tab:name}\n" +
				"\\centering\n\\begin{tabularx}{\\columnwidth}{XX}\n\\end{tabularx}\n\\end{table}\n",
		},
		{
			name: "a figure with no caption",
			latex: "\\begin{figure}[!t]\n\\centering\n\\includegraphics{a.pdf}\n" +
				"\\label{fig:name}\n\\end{figure}\n",
			want: "figure without a caption",
		},
		{
			name: "a figure with no label",
			latex: "\\begin{figure}[!t]\n\\centering\n\\includegraphics{a.pdf}\n" +
				"\\caption{A caption.}\n\\end{figure}\n",
			want: "figure without a label",
		},
		{
			name: "a caption and label with material between them",
			latex: "\\begin{table*}[!t]\n\\caption{A caption.}\n\\centering\n" +
				"\\label{tab:name}\n\\end{table*}\n",
			want: "table whose label does not follow its caption",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			found := FloatsCarryPlainCaptions(testCase.latex)
			if testCase.want == "" {
				if len(found) != 0 {
					t.Errorf("flagged a permitted float: %v", found)
				}
				return
			}
			if len(found) == 0 {
				t.Fatalf("found nothing; wanted %q", testCase.want)
			}
			if found[0].Form != testCase.want {
				t.Errorf("Form = %q, want %q", found[0].Form, testCase.want)
			}
		})
	}
}

// TestFindingsReadAsAdvice covers srd009-backport-compatibility R1.2: a
// finding says what the form breaks, because the form compiles and the cost is
// invisible without the explanation.
func TestFindingsReadAsAdvice(t *testing.T) {
	found := Forbidden("\\begin{adjustbox}{max width=\\columnwidth}\n")

	if len(found) != 1 {
		t.Fatalf("Forbidden() = %v, want one finding", found)
	}
	rendered := found[0].String()
	for _, want := range []string{"line 1", "adjustbox", "raw block"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("finding %q does not carry %q", rendered, want)
		}
	}
}
