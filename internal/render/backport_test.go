package render

import (
	"strings"
	"testing"

	"github.com/petar-djukic/md-to-tex/internal/latex"
)

// floatFixtures are the float-carrying chapters this package converts: the
// corpus srd009-backport-compatibility R4.1 asks for, one case per emitted
// construct.
//
// Every fixture is held to the whole constraint, so a float added here is
// covered by construction rather than by remembering to check it. That is also
// what srd009-backport-compatibility R4.5 asks of a change to an emitted form:
// these are ordinary tests, so changing what a renderer emits reruns them, and
// a reader that changes under us fails them without any change here.
var floatFixtures = map[string]string{
	"a figure":                    "![A caption.](fig/01-name.pdf){#fig:name}\n",
	"a wide figure":               "![A caption.](fig/02-wide.pdf){#fig:wide .wide}\n",
	"a narrowed figure":           "![A caption.](fig/03-narrow.pdf){#fig:narrow width=0.6}\n",
	"a figure captioned richly":   "![R&D on *guided* agents, cited [@du-2023].](fig/04-rich.pdf){#fig:rich}\n",
	"a table":                     surveyTable,
	"a wide table":                levelsTable,
	"a table of one body row":     "| A | B |\n|---|---|\n| 1 | 2 |\n\nTable: A caption. {#tab:small}\n",
	"a table with a rich caption": "| A | B |\n|---|---|\n| 1 | 2 |\n\nTable: A *rich* caption citing [@du-2023]. {#tab:rich}\n",
	"a chapter of both":           "# Heading\n\nProse.\n\n" + surveyTable + "\n![A caption.](fig/05-both.pdf){#fig:both}\n",
}

// TestEmittedFloatsCarryNoForbiddenForm covers
// srd009-backport-compatibility R1.1, R3.1, R3.2, R3.3, R3.4, R3.5, R3.6, AC3,
// and AC4: no fixture the float renderers emit carries a form the SRD forbids,
// the bold header row among them.
func TestEmittedFloatsCarryNoForbiddenForm(t *testing.T) {
	for name, source := range floatFixtures {
		t.Run(name, func(t *testing.T) {
			emitted := convert(t, source)

			if found := latex.Forbidden(emitted); len(found) != 0 {
				for _, form := range found {
					t.Errorf("%s carries a forbidden form: %s", name, form)
				}
				t.Logf("emitted:\n%s", emitted)
			}
		})
	}
}

// TestEmittedFloatsCarryPlainCaptions covers
// srd009-backport-compatibility R2.1, R2.2, R2.3, R2.4, and AC1: every emitted
// float carries the plain caption and label pair, in the order its kind
// states, with nothing between them.
func TestEmittedFloatsCarryPlainCaptions(t *testing.T) {
	for name, source := range floatFixtures {
		t.Run(name, func(t *testing.T) {
			emitted := convert(t, source)

			if found := latex.FloatsCarryPlainCaptions(emitted); len(found) != 0 {
				for _, form := range found {
					t.Errorf("%s: %s", name, form)
				}
				t.Logf("emitted:\n%s", emitted)
			}
		})
	}
}

// TestCaptionOrderFollowsTheFloatKind covers
// srd009-backport-compatibility R2.3: a figure captions after its include and
// a table captions before its body, which is what the manuscripts hand-write.
func TestCaptionOrderFollowsTheFloatKind(t *testing.T) {
	figure := convert(t, floatFixtures["a figure"])
	if strings.Index(figure, `\includegraphics`) > strings.Index(figure, `\caption{`) {
		t.Errorf("a figure captioned before its include:\n%s", figure)
	}

	table := convert(t, surveyTable)
	if strings.Index(table, `\caption{`) > strings.Index(table, `\begin{tabularx}`) {
		t.Errorf("a table captioned after its body:\n%s", table)
	}
}

// TestCaptionsCarryTheirInlineContent covers
// srd009-backport-compatibility R2.4: a caption carrying emphasis or a
// citation comes back carrying the same, because the reader recovers both.
func TestCaptionsCarryTheirInlineContent(t *testing.T) {
	figure := convert(t, floatFixtures["a figure captioned richly"])
	if !strings.Contains(figure, `\caption{R\&D on \emph{guided} agents, cited \cite{du-2023}.}`) {
		t.Errorf("figure caption lost its inline content:\n%s", figure)
	}

	table := convert(t, floatFixtures["a table with a rich caption"])
	if !strings.Contains(table, `\caption{A \emph{rich} caption citing \cite{du-2023}.}`) {
		t.Errorf("table caption lost its inline content:\n%s", table)
	}
}

// TestTheConstraintBindsTheEmittingComponents covers
// srd009-backport-compatibility R1.4: the constraint binds the front matter,
// the renderer core, and the float renderers, and does not bind raw LaTeX the
// author wrote.
func TestTheConstraintBindsTheEmittingComponents(t *testing.T) {
	authored := convert(t, "```{=latex}\n\\begin{adjustbox}{max width=\\columnwidth}\nauthor's own\n\\end{adjustbox}\n```\n")

	if found := latex.Forbidden(authored); len(found) == 0 {
		t.Error("the checker should still see a forbidden form in raw LaTeX")
	}
	if !strings.Contains(authored, `\begin{adjustbox}`) {
		t.Errorf("raw LaTeX was rewritten:\n%s", authored)
	}
}

// TestTypesettingEditsAreNotProse covers srd009-backport-compatibility R1.3
// and AC5: a float-placement edit is expected to be lost on the way back, and
// is not what this constraint protects.
func TestTypesettingEditsAreNotProse(t *testing.T) {
	emitted := convert(t, floatFixtures["a figure"])
	edited := strings.Replace(emitted, "[!t]", "[!htbp]", 1)

	if edited == emitted {
		t.Fatal("the fixture carries no placement specifier to edit")
	}
	for _, latexText := range []string{emitted, edited} {
		if found := latex.Forbidden(latexText); len(found) != 0 {
			t.Errorf("a placement edit produced a forbidden form: %v", found)
		}
	}
	if stripPlacement(emitted) != stripPlacement(edited) {
		t.Error("a placement edit changed something other than the placement")
	}
}

// stripPlacement removes the float placement specifiers, which is what the
// reader drops on the way back to markdown.
func stripPlacement(latexText string) string {
	return strings.NewReplacer("[!t]", "", "[!htbp]", "").Replace(latexText)
}
