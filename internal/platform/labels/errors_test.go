package labels

import (
	"testing"

	"github.com/paveltessman/pilam/internal/platform/validate"
)

func TestMessage(t *testing.T) {
	testData := map[string]struct {
		in   validate.FieldError
		want string
	}{
		"required": {
			validate.FieldError{Field: "article", Code: validate.Required},
			"Обязательно к заполнению",
		},
		"too long, many": {
			validate.FieldError{Field: "trade_name", Code: validate.TooLong, Arg: "120"},
			"Максимум 120" + nbsp + "символов",
		},
		"too long, few": {
			validate.FieldError{Field: "color", Code: validate.TooLong, Arg: "3"},
			"Максимум 3" + nbsp + "символа",
		},
		"too long, one": {
			validate.FieldError{Field: "color", Code: validate.TooLong, Arg: "1"},
			"Максимум 1" + nbsp + "символ",
		},
		"too long, teen": {
			validate.FieldError{Field: "color", Code: validate.TooLong, Arg: "11"},
			"Максимум 11" + nbsp + "символов",
		},
		"too short": {
			validate.FieldError{Field: "passwd", Code: validate.TooShort, Arg: "12"},
			"Минимум 12" + nbsp + "символов",
		},
		"too short, one": {
			validate.FieldError{Field: "passwd", Code: validate.TooShort, Arg: "1"},
			"Минимум 1" + nbsp + "символ",
		},
		"too small": {
			validate.FieldError{Field: "qty", Code: validate.TooSmall, Arg: "1"},
			"Минимум 1",
		},
		"too big": {
			validate.FieldError{Field: "qty", Code: validate.TooBig, Arg: "100000"},
			"Максимум 100" + nbsp + "000",
		},
		"not a number": {
			validate.FieldError{Field: "qty", Code: validate.NotANumber},
			"Введите число",
		},
		"not a date": {
			validate.FieldError{Field: "forecast", Code: validate.NotADate},
			"Введите дату в формате ДД.ММ.ГГГГ",
		},
		"not allowed": {
			validate.FieldError{Field: "status", Code: validate.NotAllowed},
			"Выберите значение из списка",
		},
		"immutable": {
			validate.FieldError{Field: "baseline", Code: validate.Immutable},
			"Это поле нельзя изменить",
		},
		"incorrect": {
			validate.FieldError{Field: "password", Code: validate.Incorrect},
			"Неверное значение",
		},
		"mismatch": {
			validate.FieldError{Field: "repeat_passwd", Code: validate.Mismatch},
			"Значения не совпадают",
		},
		"unknown code": {
			validate.FieldError{Field: "article", Code: "not_a_real_code"},
			unhandled,
		},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			if got := Message(tc.in); got != tc.want {
				t.Errorf("Message(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMessageWithoutItsBound(t *testing.T) {
	testData := map[string]struct {
		code validate.Code
		want string
	}{
		"too long":  {validate.TooLong, "Слишком длинный текст"},
		"too short": {validate.TooShort, "Слишком короткий текст"},
		"too small": {validate.TooSmall, "Слишком маленькое значение"},
		"too big":   {validate.TooBig, "Слишком большое значение"},
	}

	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			got := Message(validate.FieldError{Field: "qty", Code: tc.code})
			if got != tc.want {
				t.Errorf("Message = %q, want %q", got, tc.want)
			}
		})
	}
}

// A TooLong whose Arg is not a number must not print it raw.
func TestMessageWithAnUnparseableBound(t *testing.T) {
	got := Message(validate.FieldError{Field: "name", Code: validate.TooLong, Arg: "many"})
	if got != "Слишком длинный текст" {
		t.Errorf("Message = %q", got)
	}
}

func TestMessagesKeysByField(t *testing.T) {
	errs := validate.FieldErrors{
		{Field: "article", Code: validate.Required},
		{Field: "qty", Code: validate.NotANumber},
		{Field: "", Code: validate.Incorrect}, // the form as a whole
	}

	got := Messages(errs)

	want := map[string]string{
		"article": "Обязательно к заполнению",
		"qty":     "Введите число",
		"":        "Неверное значение",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d messages, want %d: %v", len(got), len(want), got)
	}
	for field, wantMsg := range want {
		if got[field] != wantMsg {
			t.Errorf("field %q: got %q, want %q", field, got[field], wantMsg)
		}
	}
}

func TestMessagesOfNothing(t *testing.T) {
	if got := Messages(nil); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

// The one message a screen supplies itself, because the generic wording would
// leak which half of the credentials was wrong.
func TestLoginFailureNamesNeitherField(t *testing.T) {
	if LoginFailed == Message(validate.FieldError{Code: validate.Incorrect}) {
		t.Error("S1 should not be using the generic message")
	}
}
