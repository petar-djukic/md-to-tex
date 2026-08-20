// Package latex holds the LaTeX-side helpers the conversion paths share.
//
// The escaper lives here rather than beside either caller because
// srd003-escaping R3.2 requires the front-matter templates and the node
// renderers to call the same function: a second replacement table drifts from
// the first, and the drift surfaces as a compile failure in a paper nobody has
// read yet.
package latex

import "strings"

// escaper holds the replacement table srd003-escaping R1.1 states: the ten
// characters LaTeX treats as markup, and nothing else.
//
// A Replacer scans its input once and never rescans what it has written, which
// is what R1.2 requires. The backslash replacement introduces backslashes of
// its own, and a second pass would escape those into textbackslash commands.
var escaper = strings.NewReplacer(
	`\`, `\textbackslash{}`,
	`&`, `\&`,
	`%`, `\%`,
	`$`, `\$`,
	`#`, `\#`,
	`_`, `\_`,
	`{`, `\{`,
	`}`, `\}`,
	`~`, `\textasciitilde{}`,
	`^`, `\textasciicircum{}`,
)

// Escape protects the ten characters that are markup in LaTeX and leaves every
// other character as written (srd003-escaping R1.1, R1.2, R1.3, R2.1).
//
// Unicode passes through untouched. Corpus titles carry Greek, accented names,
// dashes, and typographic quotation marks, and the pipeline compiles with a
// Unicode-aware engine, so transliterating them would mangle the names it
// claims to protect (srd003-escaping R2.2, R2.3).
//
// The function takes no options and returns no error, so it cannot behave one
// way for a title and another for a paragraph (srd003-escaping R3.1).
func Escape(text string) string {
	return escaper.Replace(text)
}
