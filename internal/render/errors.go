package render

import (
	"errors"

	"github.com/petar-djukic/md-to-tex/internal/cite"
)

// asUnknownKey unwraps an unknown-citation-key error, which carries the
// position the conversion error needs (srd006-citations R3.1).
func asUnknownKey(err error, target **cite.UnknownKeyError) bool {
	return errors.As(err, target)
}
