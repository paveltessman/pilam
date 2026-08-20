package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// The handlers read a submitted form through this file, so that a control of
// one kind is read the same way on every screen.
//
// A reader rejects only what the service cannot see. The service takes a
// time.Time and an ids.ID, so text that is neither is the handler's to refuse.
// Everything else — a value left empty, a name already taken — stays a rule of
// the domain.

// postedDay reads a date box.
//
// An empty box gives the zero date and no rejection: whether the form requires
// the date is the service's rule to state. A box holding text which is not a
// date also gives the zero date, so the service refuses the write, and the
// NotADate rejection names what the user must fix.
func postedDay(r *http.Request, field string) (time.Time, error) {
	raw := strings.TrimSpace(r.PostFormValue(field))
	if raw == "" {
		return time.Time{}, nil
	}

	day, err := date.Parse(raw)
	if err != nil {
		return time.Time{}, validate.Fail(field, validate.NotADate)
	}
	return day, nil
}

// postedID reads a select box that carries an identifier.
func postedID(r *http.Request, field string) (ids.ID, error) {
	raw := strings.TrimSpace(r.PostFormValue(field))
	if raw == "" {
		return ids.Nil, nil
	}

	id, err := ids.Parse(raw)
	if err != nil {
		return ids.Nil, validate.Fail(field, validate.NotAllowed)
	}
	return id, nil
}

// postedFlag reads a checkbox. An unchecked box posts nothing at all.
func postedFlag(r *http.Request, field string) bool {
	return r.PostFormValue(field) != ""
}

// rejections merges the field rejections every error given carries, keeping one
// per field: the first error that names a field wins. A nil error adds nothing.
//
// It reports false when an error carries something other than field
// rejections. Such an error is a failure rather than a refusal, and the caller
// answers 500.
func rejections(errs ...error) (validate.FieldErrors, bool) {
	var v validate.Validator
	for _, err := range errs {
		if err == nil {
			continue
		}
		if _, ok := validate.From(err); !ok {
			return nil, false
		}
		v.Merge(err)
	}

	merged, _ := validate.From(v.Err())
	return merged, true
}
