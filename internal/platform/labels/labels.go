// Package labels provides user-facing strings for UI.
package labels

import "github.com/paveltessman/pilam/internal/platform/password"

// The nav of the app header. The model list carries its own title there, so it
// has no separate name of its own.
const (
	NavExceptions = "Отставания"
	NavSeasons    = "Сезоны"
	NavDrops      = "Дропы"
	NavUsers      = "Пользователи"
	NavLogOut     = "Выйти"
)

const (
	ActionSave        = "Сохранить"
	ActionCancel      = "Отмена"
	ActionAddColorway = "Добавить цвет"
	ActionAddPhoto    = "Добавить фото"
	ActionSearch      = "Поиск"
	ActionFilter      = "Фильтры"
	ActionMarkDone    = "Отметить выполненным"
	ActionLogIn       = "Войти"
	ActionAddUser     = "Новый пользователь"
	ActionAddSeason   = "Новый сезон"
	ActionAddDrop     = "Новый дроп"
	ActionAddModel    = "Новая модель"
	ActionResetPasswd = "Сбросить пароль"
	ActionBackToList  = "Назад к списку"
)

// Common  strings that are used across ddifferent screens.
const (
	Saved = "Изменения сохранены."

	StateColumn   = "Состояние"
	StateActive   = "Активен"
	StateInactive = "Неактивен"

	// The choice a filter offers for "do not filter on this at all".
	FilterAny = "Все"
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

	// The search box above the list, and what an empty result says.
	UsersSearch     = "Поиск по имени, фамилии или почте"
	UsersSearchHint = "Имя, фамилия или почта"
	UsersNoMatch    = "Ничего не найдено"

	// The rule that keeps the last root from locking the section.
	UsersSelfLockout = "Нельзя снять доступ с самого себя. Это должен сделать другой администратор."
)

// The audit trail on the user card: the heading, the columns, and the words
// the recorded actions read as.
const (
	UsersAuditTitle = "История изменений"
	UsersAuditEmpty = "Изменений пока нет."
	UsersAuditWhen  = "Когда"
	UsersAuditWho   = "Кто"
	UsersAuditWhat  = "Что изменилось"

	UsersAuditCreated   = "Пользователь создан"
	UsersAuditFirstName = "Имя изменено"
	UsersAuditLastName  = "Фамилия изменена"
	UsersAuditRole      = "Роль изменена"
	UsersAuditOff       = "Доступ отключён"
	UsersAuditOn        = "Доступ включён"
	UsersAuditPasswd    = "Пароль изменён"
	UsersAuditReset     = "Пароль сброшен"

	// An action recorded and not named above.
	UsersAuditOther = "Изменение"
)

// The screen that shows a first password.
const (
	UsersPasswdCreated = "Пользователь создан"
	UsersPasswdReset   = "Пароль сброшен"
	UsersPasswdOnce    = "Это временный пароль. Он больше не будет показан. Скопируйте его и передайте пользователю."
	UsersPasswdChange  = "При первом входе пользователь установит новый пароль."
)

// The seasons section
const (
	SeasonsTitle     = "Сезоны"
	SeasonsNewTitle  = "Новый сезон"
	SeasonsEditTitle = "Карточка сезона"
	SeasonsEmpty     = "Сезонов пока нет."
	SeasonsName      = "Название"
	SeasonsNameHint  = "Например, S1."
	SeasonsStart     = "Начало сезона"
	SeasonsStartHint = "День, с которого сезон в работе."
)

// The drops section
const (
	DropsTitle      = "Дропы"
	DropsNewTitle   = "Новый дроп"
	DropsEditTitle  = "Карточка дропа"
	DropsEmpty      = "Дропов пока нет."
	DropsNone       = "В этом сезоне дропов пока нет."
	DropsName       = "Название"
	DropsSeason     = "Сезон"
	DropsTarget     = "Плановая дата"
	DropsTargetHint = "Планируемая дата выхода в продажу. От неё считаются сроки этапов."

	DropsNoSeason    = "Сначала создайте сезон: дроп существует внутри сезона."
	DropsSeasonFixed = "Сезон дропа изменить нельзя."
)

// The models section: the list, the create screen, and the model card.
const (
	ModelsTitle     = "Модели"
	ModelsNewTitle  = "Новая модель"
	ModelsCardTitle = "Карточка модели"
	ModelsEmpty     = "Моделей пока нет."
	ModelsNoMatch   = "По этим фильтрам ничего не найдено."
	ModelsArticle   = "Артикул"
	ModelsSeason    = "Сезон"
	ModelsDrop      = "Дроп"
	ModelsPhoto     = "Фото"

	ModelsArticleHint = "Номер артикула."
	ModelsDropHint    = "Дроп, в котором выходит модель."
	ModelsNoDrop      = "Сначала создайте сезон и дроп: модель существует внутри дропа."
	ModelsSpineFixed  = "Сезон и дроп модели изменить нельзя."
	ModelsFilterState = "Состояние"
)

// The photo strip of the model card.
const (
	ModelsPhotos      = "Фотографии"
	ModelsPhotosEmpty = "Фотографий пока нет."
	ModelsPhotosHint  = "Первое фото — обложка модели."
	ModelsPhotoCover  = "Обложка"
	ModelsPhotoOf     = "Фото модели"
	ModelsPhotoUpload = "Выберите изображения"

	ActionPhotoEarlier = "Сдвинуть влево"
	ActionPhotoLater   = "Сдвинуть вправо"
	ActionPhotoRemove  = "Удалить"

	// What a refused upload or a refused reorder reports.
	ModelsPhotoNone     = "Выберите хотя бы один файл."
	ModelsPhotoTooLarge = "Файл слишком большой."
	ModelsPhotoRejected = "Такой файл загрузить нельзя. Подойдёт JPEG, PNG, WebP или GIF."
	ModelsPhotoOrder    = "Порядок фотографий устарел. Откройте страницу заново."
)

// The failures in the http middleware chain
const (
	ErrorUnexpected = "Что-то пошло не так. Попробуйте ещё раз."
	ErrorForbidden  = "Запрос отклонён."
)
