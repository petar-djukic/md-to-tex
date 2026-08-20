package render

import "strings"

// slug derives a heading's identifier from its text: lowercased, every run of
// characters outside the ASCII letters and digits collapsed to a single
// hyphen, leading and trailing hyphens removed (srd-2-renderer-core R3.5).
//
// The manuscripts reference sections by this slug, which pandoc derived for
// them, so the rule reproduces pandoc's result for the headings they carry.
// The two differ where punctuation sits inside a word without a space beside
// it - pandoc drops an apostrophe where this rule yields a hyphen - and no
// heading in the corpus does that.
func slug(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))

	pendingHyphen := false
	for _, character := range text {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			if pendingHyphen && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			pendingHyphen = false
			builder.WriteRune(character)
		case character >= 'A' && character <= 'Z':
			if pendingHyphen && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			pendingHyphen = false
			builder.WriteRune(character - 'A' + 'a')
		default:
			pendingHyphen = true
		}
	}
	return builder.String()
}

// sectioningCommand is the IEEEtran command for a heading level
// (srd-2-renderer-core R3.1). A level with no command is not this function's
// to report: R3.2 makes it an error where the heading is rendered.
func sectioningCommand(level int) (string, bool) {
	switch level {
	case 1:
		return `\section`, true
	case 2:
		return `\subsection`, true
	case 3:
		return `\subsubsection`, true
	case 4:
		return `\paragraph`, true
	}
	return "", false
}
