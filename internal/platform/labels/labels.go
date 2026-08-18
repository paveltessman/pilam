// Package labels provides user-facing strings for UI.
package labels

import "github.com/paveltessman/pilam/internal/platform/password"

const (
	NavBoard      = "Сезон"
	NavExceptions = "Отставания"
	NavDrops      = "Дропы"
	NavUsers      = "Пользователи"
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
	ActionAddUser     = "Новый пользователь"
	ActionResetPasswd = "Сбросить пароль"
	ActionBackToList  = "Назад к списку"
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

// The users section
const (
	UsersTitle      = "Пользователи"
	UsersNewTitle   = "Новый пользователь"
	UsersEditTitle  = "Карточка пользователя"
	UsersEmpty      = "Пользователей пока нет."
	UsersEmail      = "Электронная почта"
	UsersName       = "Имя"
	UsersFirstName  = "Имя"
	UsersLastName   = "Фамилия"
	UsersRole       = "Роль"
	UsersState      = "Доступ"
	UsersLastChange = "Изменён"
	UsersActive     = "Активен"
	UsersInactive   = "Отключён"
	UsersRoleMember = "Сотрудник"
	UsersRoleRoot   = "Администратор"

	// The rule that keeps the last root from locking the section.
	UsersSelfLockout = "Нельзя снять доступ с самого себя. Это должен сделать другой администратор."
)

// The screen that shows a first password.
const (
	UsersPasswdCreated = "Пользователь создан"
	UsersPasswdReset   = "Пароль сброшен"
	UsersPasswdOnce    = "Это временный пароль. Он больше не будет показан. Скопируйте его и передайте пользователю."
	UsersPasswdChange  = "При первом входе пользователь установит новый пароль."
)

// The failures in the http middleware chain
const (
	ErrorUnexpected = "Что-то пошло не так. Попробуйте ещё раз."
	ErrorForbidden  = "Запрос отклонён."
)
