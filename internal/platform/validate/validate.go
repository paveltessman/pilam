// Package validate is the single vocabulary of input rejections.
//
// A rejection is a field name plus a code: "article: required". No user-facing
// text is produced here.
//
// There are two ways in. A handler checking a submitted form accumulates with a
// Validator and reports every bad field at once. A domain package refusing a
// single value — a wrong password, an edit to a frozen baseline — returns Fail.
// Both produce FieldErrors

package validate

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Code is why a field was rejected.
type Code string

const (
	// The field was left empty.
	Required Code = "required"

	// Text outside the length the field allows. Arg is the limit, in
	// characters.
	TooLong  Code = "too_long"
	TooShort Code = "too_short"

	// A number outside its bounds. Arg is the bound.
	TooSmall Code = "too_small"
	TooBig   Code = "too_big"

	// The input did not parse.
	NotANumber Code = "not_a_number"
	NotADate   Code = "not_a_date"

	// A value outside the vocabulary the field is backed by.
	NotAllowed Code = "not_allowed"

	// A field that exists but cannot be changed.
	Immutable Code = "immutable"

	// Well-formed but wrong. The login password.
	Incorrect Code = "incorrect"

	// Two fields that must carry the same value do not. The repeated password.
	Mismatch Code = "mismatch"
)

// FieldError is one rejection: which field, why, and the bound it was measured
// against when the code has one.
//
// Field is empty for a rejection that belongs to the form as a whole rather than
// to any one input.
//
// Comparable, so tests and the view layer can compare values directly.
type FieldError struct {
	Field string
	Code  Code
	Arg   string
}

// At most one error per field, in the order the fields were checked.
type FieldErrors []FieldError

// Error renders the set for a log line. It is not what the user reads: the UI
// message comes from labels, per code.
func (fe FieldErrors) Error() string {
	parts := make([]string, len(fe))
	for i, e := range fe {
		part := string(e.Code)
		if e.Field != "" {
			part = e.Field + ": " + part
		}
		if e.Arg != "" {
			part += "(" + e.Arg + ")"
		}
		parts[i] = part
	}
	return "invalid input: " + strings.Join(parts, "; ")
}

// Get returns the error recorded against field, and whether there was one.
func (fe FieldErrors) Get(field string) (FieldError, bool) {
	for _, e := range fe {
		if e.Field == field {
			return e, true
		}
	}
	return FieldError{}, false
}

// From extracts the field errors from err, reporting whether it carried any.
func From(err error) (FieldErrors, bool) {
	return errors.AsType[FieldErrors](err)
}

// Fail is the one-shot form, for a domain package refusing a single value:
//
//	return validate.Fail("baseline", validate.Immutable)
func Fail(field string, code Code) error {
	return FieldErrors{{Field: field, Code: code}}
}

// Validator accumulates rejections over the fields of one form, so that a
// submission with three bad fields comes back with three messages rather than
// one at a time.
//
// The zero value is ready to use. Not safe for concurrent use: it belongs to
// the request handling one form.
type Validator struct {
	errs FieldErrors
}

// Add records a code against a field, unless the field already carries one.
//
// One error per field, the first one recorded.
//
// This is also the way to record a code carrying an Arg that the checks below
// do not cover.
func (v *Validator) Add(field string, code Code, arg string) {
	if v.Has(field) {
		return
	}
	v.errs = append(v.errs, FieldError{Field: field, Code: code, Arg: arg})
}

// Check records code against field when ok is false.
//
// It is the primitive the other checks are built from, e.g:
//
//	d, err := date.Parse(raw)
//	v.Check(err == nil, "forecast", validate.NotADate)
func (v *Validator) Check(ok bool, field string, code Code) bool {
	if !ok {
		v.Add(field, code, "")
	}
	return !v.Has(field)
}

// Required rejects an empty value. Whitespace counts as empty, so a field
// holding a single space is not a filled-in field.
func (v *Validator) Required(field, value string) bool {
	return v.Check(strings.TrimSpace(value) != "", field, Required)
}

// MaxLen rejects text longer than limit characters (characters, not bytes).
func (v *Validator) MaxLen(field, value string, limit int) bool {
	if utf8.RuneCountInString(value) > limit {
		v.Add(field, TooLong, strconv.Itoa(limit))
	}
	return !v.Has(field)
}

// MinLen rejects text shorter than limit characters (characters, not bytes).
func (v *Validator) MinLen(field, value string, limit int) bool {
	if utf8.RuneCountInString(value) < limit {
		v.Add(field, TooShort, strconv.Itoa(limit))
	}
	return !v.Has(field)
}

// AtLeast rejects n below limit. It is the bound of a number, not of text:
// MinLen is the one for text.
func (v *Validator) AtLeast(field string, n, limit int) bool {
	if n < limit {
		v.Add(field, TooSmall, strconv.Itoa(limit))
	}
	return !v.Has(field)
}

// AtMost rejects n above limit.
func (v *Validator) AtMost(field string, n, limit int) bool {
	if n > limit {
		v.Add(field, TooBig, strconv.Itoa(limit))
	}
	return !v.Has(field)
}

// OneOf rejects a value outside of the allowed set.
func (v *Validator) OneOf(field, value string, allowed ...string) bool {
	return v.Check(slices.Contains(allowed, value), field, NotAllowed)
}

// Has reports whether field has already been rejected.
func (v *Validator) Has(field string) bool {
	_, found := v.errs.Get(field)
	return found
}

// Err returns everything rejected so far as one error, or nil.
func (v *Validator) Err() error {
	if len(v.errs) == 0 {
		return nil
	}
	return v.errs
}

// Merge extracts the field errors from err and adds them to the validator.
func (v *Validator) Merge(err error) {
	errs, _ := From(err)
	for _, e := range errs {
		v.Add(e.Field, e.Code, e.Arg)
	}
}
