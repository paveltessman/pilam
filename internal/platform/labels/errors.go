package labels

import (
	"strconv"

	"github.com/paveltessman/pilam/internal/platform/validate"
)

// If a code from 'platform/validate' is not handled here
const unhandled = "Некорректное значение"

// Message renders a FieldError to a user-facing sentence.
func Message(e validate.FieldError) string {
	switch e.Code {
	case validate.Required:
		return "Обязательно к заполнению"

	case validate.TooLong:
		if limit, err := strconv.Atoi(e.Arg); err == nil {
			return "Максимум " + Chars(limit)
		}
		return "Слишком длинный текст"

	case validate.TooShort:
		if limit, err := strconv.Atoi(e.Arg); err == nil {
			return "Минимум " + Chars(limit)
		}
		return "Слишком короткий текст"

	case validate.TooSmall:
		if e.Arg == "" {
			return "Слишком маленькое значение"
		}
		return "Минимум " + Decimal(e.Arg)

	case validate.TooBig:
		if e.Arg == "" {
			return "Слишком большое значение"
		}
		return "Максимум " + Decimal(e.Arg)

	case validate.NotANumber:
		return "Введите число"

	case validate.NotADate:
		return "Введите дату в формате ДД.ММ.ГГГГ"

	case validate.NotAllowed:
		return "Выберите значение из списка"

	case validate.Immutable:
		return "Это поле нельзя изменить"

	case validate.Incorrect:
		return "Неверное значение"

	case validate.Mismatch:
		return "Значения не совпадают"

	case validate.Taken:
		return "Значение уже занято"

	default:
		// A code added to validate and not added here.
		// Distinct from every deliberate message above, so a test can tell a
		// code that fell through from one that was handled.
		return unhandled
	}
}

// Messages is a whole set of rejections keyed by the field each belongs to.
//
// The form-level rejection — the one validate leaves with an empty field — is
// under the empty key.
func Messages(errs validate.FieldErrors) map[string]string {
	byField := make(map[string]string, len(errs))
	for _, e := range errs {
		byField[e.Field] = Message(e)
	}
	return byField
}
