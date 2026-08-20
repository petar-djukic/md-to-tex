package latex

import (
	"fmt"
	"regexp"
	"strings"
)

// ForbiddenForm is one construct srd009-backport-compatibility forbids, found
// in emitted LaTeX.
type ForbiddenForm struct {
	// Form names the construct, as the SRD names it.
	Form string
	// Line is where it appears, counting from one.
	Line int
	// Why says what the form breaks, so a reader fixing it knows what the
	// rule protects rather than only that a rule exists.
	Why string
}

func (f ForbiddenForm) String() string {
	return fmt.Sprintf("line %d: %s: %s", f.Line, f.Form, f.Why)
}

// forbidden holds the forms srd009-backport-compatibility R3 lists, each with
// what it costs.
//
// The test is not whether a form compiles. Every one of these compiles; each
// reads back from the pipeline's LaTeX reader as something other than the
// prose an author edited, which is the failure this list exists to prevent
// (srd009-backport-compatibility R1.1, R1.2, R3.6).
var forbidden = []struct {
	form    string
	pattern *regexp.Regexp
	why     string
}{
	{
		form:    "adjustbox",
		pattern: regexp.MustCompile(`\\begin\{adjustbox\}`),
		why: "a wrapper the reader does not know, so the include, the caption, and the label " +
			"come back as one raw block (R3.1)",
	},
	{
		form:    "refstepcounter",
		pattern: regexp.MustCompile(`\\refstepcounter\{(table|figure)\}`),
		why: "a caption assembled by hand reads back as bold prose that no longer looks like a " +
			"caption, so a caption edit merges into the body or is dropped (R3.2)",
	},
	{
		form:    "a bold header row",
		pattern: regexp.MustCompile(`(?m)^\\textbf\{[^}]*\}( *& *\\textbf\{[^}]*\})+ *\\\\`),
		why: "the reader recovers the bold as emphasis around every header cell, so a backported " +
			"table differs in cells nobody edited (R3.3)",
	},
	{
		form:    "strip",
		pattern: regexp.MustCompile(`\\begin\{strip\}`),
		why: "a non-float spanning mechanism reads back as raw, and reserves no space, so it " +
			"lands on the column text (R3.4)",
	},
	{
		form:    "longtable",
		pattern: regexp.MustCompile(`\\begin\{longtable\}`),
		why:     "a two-column class has nowhere to put a page-breaking table (R3.5, srd005-tables R5.1)",
	},
}

// Forbidden reports every construct srd009-backport-compatibility forbids in
// the given LaTeX, with the line it sits on.
//
// The library calls this over its own output; the consuming pipeline can call
// it over a fragment an author has hand-edited, which is where a forbidden
// form is most likely to arrive.
func Forbidden(latex string) []ForbiddenForm {
	var found []ForbiddenForm
	for number, line := range strings.Split(latex, "\n") {
		for _, candidate := range forbidden {
			if candidate.pattern.MatchString(line) {
				found = append(found, ForbiddenForm{
					Form: candidate.form,
					Line: number + 1,
					Why:  candidate.why,
				})
			}
		}
	}
	return found
}

// floatEnvironment opens a float, and captionThenLabel is the pair every float
// must carry in that order (srd009-backport-compatibility R2.1).
var (
	floatEnvironment = regexp.MustCompile(`\\begin\{(figure\*?|table\*?)\}`)
	captionCommand   = regexp.MustCompile(`^\\caption\{`)
	labelCommand     = regexp.MustCompile(`^\\label\{`)
)

// FloatsCarryPlainCaptions reports every float in the given LaTeX whose
// caption and label are not the plain pair the SRD permits, in the order it
// states for that float kind (srd009-backport-compatibility R2.1, R2.3).
//
// A figure captions after its include and a table captions before its body, so
// the check is on the pair and its order rather than on a fixed line.
func FloatsCarryPlainCaptions(latex string) []ForbiddenForm {
	var found []ForbiddenForm
	lines := strings.Split(latex, "\n")

	for number, line := range lines {
		match := floatEnvironment.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		kind := strings.TrimSuffix(match[1], "*")

		captionAt, labelAt := -1, -1
		for offset := number + 1; offset < len(lines); offset++ {
			if strings.HasPrefix(lines[offset], `\end{`+match[1]+`}`) {
				break
			}
			if captionAt < 0 && captionCommand.MatchString(lines[offset]) {
				captionAt = offset
			}
			if labelAt < 0 && labelCommand.MatchString(lines[offset]) {
				labelAt = offset
			}
		}

		switch {
		case captionAt < 0:
			found = append(found, ForbiddenForm{Form: kind + " without a caption", Line: number + 1,
				Why: "a float LaTeX places away from its text is unidentifiable without one (R2.1)"})
		case labelAt < 0:
			found = append(found, ForbiddenForm{Form: kind + " without a label", Line: number + 1,
				Why: "nothing can reference it (R2.1)"})
		case labelAt != captionAt+1:
			found = append(found, ForbiddenForm{Form: kind + " whose label does not follow its caption",
				Line: number + 1,
				Why:  "the reader recovers the pair, and only the pair, as a caption (R2.1)"})
		}
	}
	return found
}
