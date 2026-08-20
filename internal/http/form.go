package http

import (
	"fmt"
	"net/http"
	"strconv"
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

// postedOrder reads a control that posts a list of identifiers, such as the
// photo strip of the model card.
//
// One value that is not an identifier refuses the whole list, because a partial
// order is not an order.
func postedOrder(r *http.Request, field string) ([]ids.ID, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("%w: the request carries no readable form: %w", ids.ErrInvalid, err)
	}

	raw := r.PostForm[field]
	order := make([]ids.ID, len(raw))
	for i, value := range raw {
		id, err := ids.Parse(strings.TrimSpace(value))
		if err != nil {
			return nil, err
		}
		order[i] = id
	}
	return order, nil
}

// parsedID reads an identifier the user did not type: a query parameter of a
// filter, or a select the form posts back.
//
// Anything that is not an identifier gives Nil. A filter is not a write, so a
// value naming nothing filters nothing out.
func parsedID(raw string) ids.ID {
	id, err := ids.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ids.Nil
	}
	return id
}

// flagOf reads a filter over a flag. It points at the flag for "true" and
// "false", and it is nil for anything else, which keeps both.
func flagOf(raw string) *bool {
	flag, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	return &flag
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
