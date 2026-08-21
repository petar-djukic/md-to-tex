package cite

import (
	"fmt"

	"github.com/yuin/goldmark/parser"
)

// Malformed is a bracketed run that opened with an at sign and did not parse
// as a citation (srd006-citations R4.2).
type Malformed struct {
	// Offset is where the run began in the source, so the caller can name the
	// line an author has to go to.
	Offset int

	// Run is what the source held there, trimmed to what fits a message.
	Run string
}

func (m Malformed) Error() string {
	return fmt.Sprintf("%s does not parse as a citation", m.Run)
}

// malformedKey is where the parser leaves what it declined. Parsing and
// rendering are separate passes, and an inline parser cannot fail a
// conversion on its own; the renderer core reads this after the parse and
// reports the position (srd006-citations R4.2, srd002-renderer-core R6.4).
var malformedKey = parser.NewContextKey()

// recordMalformed notes a run the parser declined, keeping the earliest so a
// caller reporting one failure reports the first an author would find.
func recordMalformed(pc parser.Context, run Malformed) {
	if existing, ok := pc.Get(malformedKey).(Malformed); ok && existing.Offset <= run.Offset {
		return
	}
	pc.Set(malformedKey, run)
}

// MalformedIn returns the run the parser declined during this parse, if any.
//
// It is read after parsing rather than during it, because a citation that does
// not parse is not the inline parser's to fail: the parser declines, the
// parsers behind it get their turn, and nothing else claims a run that opens
// with a bracket and an at sign.
func MalformedIn(pc parser.Context) (Malformed, bool) {
	run, ok := pc.Get(malformedKey).(Malformed)
	return run, ok
}
