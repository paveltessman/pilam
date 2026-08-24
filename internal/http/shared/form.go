package shared

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

// The handlers read a submitted form through this file.

// PostedDay reads a date box.
//
// An empty box gives the zero date and no rejection: the service decides
// whether the date is required or not.
func PostedDay(r *http.Request, field string) (time.Time, error) {
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

// PostedNumber reads a number box.
//
// An empty box gives zero and reports false: the caller decides whether a box
// that was left empty is a value or not.
func PostedNumber(r *http.Request, field string) (int, bool, error) {
	raw := strings.TrimSpace(r.PostFormValue(field))
	if raw == "" {
		return 0, false, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, validate.Fail(field, validate.NotANumber)
	}
	return n, true, nil
}

// PostedID reads a select box that carries an identifier.
func PostedID(r *http.Request, field string) (ids.ID, error) {
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

// PostedOrder reads a control that posts a list of identifiers, such as the
// photo strip of the model card.
//
// One value that is not an identifier refuses the whole list, because a partial
// order is not an order.
func PostedOrder(r *http.Request, field string) ([]ids.ID, error) {
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

// ParsedID reads an identifier the user did not type: a query parameter of a
// filter, or a select the form posts back.
//
// Anything that is not an identifier gives Nil.
func ParsedID(raw string) ids.ID {
	id, err := ids.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ids.Nil
	}
	return id
}

// FlagOf reads a filter over a flag. It points at the flag for "true" and
// "false", and it is nil for anything else.
func FlagOf(raw string) *bool {
	flag, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	return &flag
}

// PostedFlag reads a checkbox. An unchecked box posts nothing at all.
func PostedFlag(r *http.Request, field string) bool {
	return r.PostFormValue(field) != ""
}

// Rejections merges the field rejections every error given carries, keeping one
// per field: the first error that names a field wins. A nil error adds nothing.
//
// It reports false when an error carries something other than field
// Rejections.
func Rejections(errs ...error) (validate.FieldErrors, bool) {
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
