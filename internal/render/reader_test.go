package render

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// readBack converts emitted LaTeX to markdown through the reader the
// consuming pipeline runs, which is what decides whether an edit made on the
// LaTeX side reaches the markdown (srd009-backport-compatibility R4.2).
//
// The reader is not this library's dependency, so a tree without it skips
// rather than fails (srd009-backport-compatibility R4.4).
func readBack(t *testing.T, latexText string) string {
	t.Helper()
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("the pipeline's reader is absent; these assertions need it")
	}

	command := exec.Command("pandoc", "--from", "latex", "--to", "markdown", "--wrap=none")
	command.Stdin = strings.NewReader(latexText)
	var out, errors bytes.Buffer
	command.Stdout = &out
	command.Stderr = &errors
	if err := command.Run(); err != nil {
		t.Fatalf("read back: %v: %s", err, errors.String())
	}
	return stripLaTeXAttributes(out.String())
}

// latexAttribute is the float placement and friends, which the reader keeps as
// an attribute rather than dropping. The consuming pipeline strips these
// before it compares two sides, so a test that compares must strip them too
// (paperkit backport.go stripLatexAttributes).
var latexAttribute = regexp.MustCompile(` data-latex[a-z-]*="[^"]*"`)

func stripLaTeXAttributes(markdown string) string {
	return latexAttribute.ReplaceAllString(markdown, "")
}

// TestEmittedFloatsReadBackAsProse covers srd009-backport-compatibility R1.1,
// R4.1, R4.2, R4.4, AC1, and AC6: every fixture's caption and cell text comes
// back as prose rather than inside a raw block, and the whole set skips
// cleanly where the reader is absent.
func TestEmittedFloatsReadBackAsProse(t *testing.T) {
	cases := []struct {
		fixture string
		wants   []string
	}{
		{"a figure", []string{"A caption."}},
		{"a wide figure", []string{"A caption."}},
		{"a table", []string{
			"Adjacent surveys and what this article adds over each.",
			"Zero-touch automation for 5G/6G",
			"This article",
		}},
		{"a wide table", []string{"Fault management"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.fixture, func(t *testing.T) {
			markdown := readBack(t, convert(t, floatFixtures[testCase.fixture]))

			for _, want := range testCase.wants {
				if !strings.Contains(markdown, want) {
					t.Errorf("%q did not come back as prose:\n%s", want, markdown)
				}
			}
			if strings.Contains(markdown, "```{=latex}") {
				t.Errorf("the float came back as a raw block:\n%s", markdown)
			}
		})
	}
}

// TestACaptionEditComesBackAsProse covers srd009-backport-compatibility R2.1,
// R4.3, and AC2: a caption edited in the emitted LaTeX differs after the round
// trip, and differs in the caption rather than everywhere.
func TestACaptionEditComesBackAsProse(t *testing.T) {
	for _, fixture := range []string{"a figure", "a table"} {
		t.Run(fixture, func(t *testing.T) {
			emitted := convert(t, floatFixtures[fixture])
			edited := strings.Replace(emitted,
				`\caption{A caption.}`, `\caption{An edited caption.}`, 1)
			if edited == emitted {
				edited = strings.Replace(emitted,
					`\caption{Adjacent surveys and what this article adds over each.}`,
					`\caption{Adjacent surveys, revisited.}`, 1)
			}
			if edited == emitted {
				t.Fatal("the fixture carries no caption to edit")
			}

			before, after := readBack(t, emitted), readBack(t, edited)
			if before == after {
				t.Errorf("a caption edit did not survive the round trip:\n%s", after)
			}
			if difference := differingLines(before, after); difference > 2 {
				t.Errorf("a caption edit changed %d lines, want the caption alone:\n%s", difference, after)
			}
		})
	}
}

// TestAPlacementEditComesBackAsNothing covers
// srd009-backport-compatibility R1.3 and AC5: typesetting is expected to be
// lost on the way back, which is the boundary this constraint draws.
func TestAPlacementEditComesBackAsNothing(t *testing.T) {
	emitted := convert(t, floatFixtures["a figure"])
	edited := strings.Replace(emitted, "[!t]", "[!htbp]", 1)

	if readBack(t, emitted) != readBack(t, edited) {
		t.Error("a placement edit produced a prose difference")
	}
}

// differingLines counts the lines two documents do not share.
func differingLines(before, after string) int {
	previous := map[string]int{}
	for _, line := range strings.Split(before, "\n") {
		previous[strings.TrimSpace(line)]++
	}
	count := 0
	for _, line := range strings.Split(after, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if previous[trimmed] > 0 {
			previous[trimmed]--
			continue
		}
		count++
	}
	return count
}

// TestAWideTableLosesItsCaptionOnTheWayBack records a conflict between two
// requirements, found by running the reader rather than by reading the SRDs.
//
// srd005-tables R4.5 sends a spanning table to the starred environment, and
// srd009-backport-compatibility R1.1 requires a caption to come back as prose.
// The reader knows the plain table environment and recovers its caption as a
// markdown table caption; it treats table* as an unknown div and drops the
// caption entirely. The starred figure environment does not have the problem,
// because the reader renders it as an HTML figure with its figcaption intact.
//
// The two requirements cannot both hold for a wide table under this reader.
// Which one gives is a decision for the SRDs, filed as GH-34, so this test
// records the finding rather than asserting either answer.
func TestAWideTableLosesItsCaptionOnTheWayBack(t *testing.T) {
	markdown := readBack(t, convert(t, floatFixtures["a wide table"]))

	if strings.Contains(markdown, "Autonomy levels by authorship and approval.") {
		t.Fatal("the reader now recovers a table* caption; GH-34 is resolved and this test should assert it")
	}
	t.Logf("a wide table's caption does not survive the round trip (GH-34):\n%s", markdown)
}
