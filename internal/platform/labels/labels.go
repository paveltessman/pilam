// Package labels provides user-facing strings for UI.
package labels

import "github.com/paveltessman/pilam/internal/platform/password"

const (
	NavBoard      = "Сезон"
	NavExceptions = "Отставания"
	NavDrops      = "Дропы"
	NavLogOut     = "Выйти"
)

const (
	ActionSave        = "Сохранить"
	ActionCancel      = "Отмена"
	ActionNewStyle    = "Новая модель"
	ActionAddColorway = "Добавить цвет"
	ActionAddPhoto    = "Добавить фото"
	ActionSearch      = "Поиск"
	ActionFilter      = "Фильтры"
	ActionMarkDone    = "Отметить выполненным"
	ActionLogIn       = "Войти"
)

const (
	LoginTitle    = "Вход"
	LoginEmail    = "Электронная почта"
	LoginPassword = "Пароль"
	LoginFailed   = "Неверная почта или пароль"
)

const (
	PasswordTitle   = "Смена пароля"
	PasswordExpired = "Смените пароль, чтобы продолжить работу."
	PasswordCurrent = "Текущий пароль"
	PasswordNew     = "Новый пароль"
	PasswordRepeat  = "Новый пароль ещё раз"
)

// PasswordHint states the length rule before the user meets it as a rejection.
var PasswordHint = "Не менее " + Chars(password.MinLen) + "."

// The failures in the http middleware chain
const (
	ErrorUnexpected = "Что-то пошло не так. Попробуйте ещё раз."
	ErrorForbidden  = "Запрос отклонён."
)
