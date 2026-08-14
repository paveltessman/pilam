package validate_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/validate"
)

// err should be a proper nil for "if err != nil" to work
func TestErrIsNilWhenNothingFailed(t *testing.T) {
	var v validate.Validator
	v.Required("article", "ART-001")

	if err := v.Err(); err != nil {
		t.Errorf("Err = %v (%T), want nil", err, err)
	}
}

func TestZeroValidatorIsUsable(t *testing.T) {
	var v validate.Validator

	if v.Has("article") {
		t.Error("a fresh Validator reports an error on a field nothing touched")
	}
	v.Required("article", "")
	if !v.Has("article") {
		t.Error("Required on an empty value recorded nothing")
	}
}

func TestRequired(t *testing.T) {
	testData := map[string]struct {
		value  string
		reject bool
	}{
		"empty":            {"", true},
		"spaces":           {"   ", true},
		"tab and newline":  {"\t\n", true},
		"text":             {"Coat", false},
		"text with spaces": {"  Coat  ", false},
		"zero":             {"0", false},
	}

	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			var v validate.Validator
			v.Required("name", tc.value)

			if got := v.Has("name"); got != tc.reject {
				t.Errorf("Required(%q) rejected = %v, want %v", tc.value, got, tc.reject)
			}
		})
	}
}

// Cyrillic is two bytes a character. Counting bytes would reject a name at half
// the allowance the field advertises.
func TestMaxLenCountsCharactersNotBytes(t *testing.T) {
	const name = "Пальто" // 6 characters, 12 bytes

	var v validate.Validator
	v.MaxLen("name", name, 6)

	if v.Has("name") {
		t.Errorf("MaxLen(%q, 6) rejected a name of exactly 6 characters", name)
	}

	var over validate.Validator
	over.MaxLen("name", name, 5)

	got := errFor(t, &over, "name")
	want := (validate.FieldError{Field: "name", Code: validate.TooLong, Arg: "5"})
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBounds(t *testing.T) {
	testData := map[string]struct {
		check func(*validate.Validator)
		want  validate.FieldError
	}{
		"below the minimum": {
			func(v *validate.Validator) { v.AtLeast("qty", -1, 0) },
			validate.FieldError{Field: "qty", Code: validate.TooSmall, Arg: "0"},
		},
		"above the maximum": {
			func(v *validate.Validator) { v.AtMost("qty", 100_001, 100_000) },
			validate.FieldError{Field: "qty", Code: validate.TooBig, Arg: "100000"},
		},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			var v validate.Validator
			tc.check(&v)

			fe, ok := validate.From(v.Err())
			if !ok {
				t.Fatal("nothing was rejected")
			}
			if got, _ := fe.Get("qty"); got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestBoundsAcceptTheLimitItself(t *testing.T) {
	var v validate.Validator
	v.AtLeast("low", 0, 0)
	v.AtMost("high", 100, 100)

	if err := v.Err(); err != nil {
		t.Errorf("the bounds are exclusive, want inclusive: %v", err)
	}
}

func TestOneOf(t *testing.T) {
	vocabulary := []string{"top", "bottom", "accessory"}

	var v validate.Validator
	v.OneOf("class", "bottom", vocabulary...)
	v.OneOf("category", "hat", vocabulary...)

	if v.Has("class") {
		t.Error("OneOf rejected a value that is in the vocabulary")
	}
	if got := errFor(t, &v, "category"); got.Code != validate.NotAllowed {
		t.Errorf("code = %q, want %q", got.Code, validate.NotAllowed)
	}
}

func TestCheckReportsAnOutsideFailure(t *testing.T) {
	err := errors.New("not a date")
	fieldName := "forecast"

	var v validate.Validator
	passed := v.Check(err == nil, fieldName, validate.NotADate)

	if passed {
		t.Error("Check reported the field clean after recording against it")
	}

	got := errFor(t, &v, fieldName)
	if got.Code != validate.NotADate {
		t.Errorf("code = %q, want %q", got.Code, validate.NotADate)
	}
}

// One message per input, and it is the first failure
func TestOnlyTheFirstErrorPerFieldIsKept(t *testing.T) {
	var v validate.Validator
	fieldName := "article"

	v.Required(fieldName, "")
	v.MaxLen(fieldName, "", 5)
	v.Check(false, fieldName, validate.NotAllowed)
	v.Add(fieldName, validate.Immutable, "")

	fe, _ := validate.From(v.Err())
	if len(fe) != 1 {
		t.Fatalf("got %d errors on one field, want 1: %v", len(fe), fe)
	}
	if fe[0].Code != validate.Required {
		t.Errorf("kept %q, want the first failure %q", fe[0].Code, validate.Required)
	}
}

// A field that fails once poisons the return value of later checks on it, so a
// caller can guard expensive work on any of them.
func TestChecksReportWhetherTheFieldIsClean(t *testing.T) {
	var v validate.Validator
	fieldName := "name"
	fieldValue := "Coat"

	if !v.Required(fieldName, fieldValue) {
		t.Error("Required reported a good value as unclean")
	}
	v.Add(fieldName, validate.Immutable, "")

	if v.MaxLen(fieldName, fieldValue, 100) {
		t.Error("a passing check reported a field that had already failed as clean")
	}
}

func TestErrCollectsEveryFieldInOrder(t *testing.T) {
	var v validate.Validator

	fieldName1 := "name1"
	fieldName2 := "name2"
	fieldName3 := "name3"

	v.Required(fieldName1, "")
	v.Required(fieldName2, "Coat")
	v.AtMost(fieldName3, 10, 5)

	fe, ok := validate.From(v.Err())
	if !ok {
		t.Fatal("Err returned no field errors")
	}

	want := validate.FieldErrors{
		{Field: fieldName1, Code: validate.Required},
		{Field: fieldName3, Code: validate.TooBig, Arg: "5"},
	}
	if len(fe) != len(want) {
		t.Fatalf("got %v, want %v", fe, want)
	}
	for i := range want {
		if fe[i] != want[i] {
			t.Errorf("error %d = %+v, want %+v", i, fe[i], want[i])
		}
	}
}

func TestFail(t *testing.T) {
	fieldName := "something"
	err := validate.Fail(fieldName, validate.Incorrect)

	fe, ok := validate.From(err)
	if !ok {
		t.Fatalf("Fail returned %T, want field errors", err)
	}
	want := (validate.FieldErrors{{Field: fieldName, Code: validate.Incorrect}})
	if len(fe) != 1 || fe[0] != want[0] {
		t.Errorf("got %v, want %v", fe, want)
	}
}

// The domain wraps as it returns; the HTTP layer still has to recognise a
// rejection several layers down as one.
func TestFromUnwraps(t *testing.T) {
	fieldName := "baseline"
	err := fmt.Errorf("moving the forecast: %w", validate.Fail(fieldName, validate.Immutable))

	fe, ok := validate.From(err)
	if !ok {
		t.Fatal("From did not find the field errors through the wrapping")
	}
	if got, _ := fe.Get(fieldName); got.Code != validate.Immutable {
		t.Errorf("code = %q, want %q", got.Code, validate.Immutable)
	}
}

func TestFromRejectsOtherErrors(t *testing.T) {
	if fe, ok := validate.From(errors.New("the database is down")); ok {
		t.Errorf("From reported %v for an unrelated error", fe)
	}
	if fe, ok := validate.From(nil); ok {
		t.Errorf("From reported %v for a nil error", fe)
	}
}

func TestGetMissingField(t *testing.T) {
	fe := validate.FieldErrors{{Field: "article", Code: validate.Required}}

	got, found := fe.Get("name")
	if found {
		t.Errorf("Get found %+v on a field that was not rejected", got)
	}
	if (got != validate.FieldError{}) {
		t.Errorf("Get returned %+v beside found=false, want the zero value", got)
	}
}

func TestErrorMessage(t *testing.T) {
	fe := validate.FieldErrors{
		{Field: "article", Code: validate.Required},
		{Field: "name", Code: validate.TooLong, Arg: "120"},
		{Code: validate.Incorrect},
	}

	const want = "invalid input: article: required; name: too_long(120); incorrect"
	if got := fe.Error(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// errFor is the error recorded against field, failing the test if there is none.
func errFor(t *testing.T, v *validate.Validator, field string) validate.FieldError {
	t.Helper()

	fe, ok := validate.From(v.Err())
	if !ok {
		t.Fatalf("nothing was rejected, want an error on %q", field)
	}
	e, ok := fe.Get(field)
	if !ok {
		t.Fatalf("no error on %q, got %v", field, fe)
	}
	return e
}
