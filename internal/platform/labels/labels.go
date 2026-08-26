// Package labels provides user-facing strings for UI.
package labels

import "github.com/paveltessman/pilam/internal/platform/password"

// The nav of the app header. The model list and the milestone list carry their
// own titles there, so neither has a separate name of its own.
const (
	NavSeasons    = "Сезоны"
	NavMilestones = "Этапы"
	NavDrops      = "Дропы"
	NavUsers      = "Пользователи"
	NavLogOut     = "Выйти"
)

const (
	ActionSave             = "Сохранить"
	ActionCancel           = "Отмена"
	ActionAddColorway      = "Добавить цвет"
	ActionAddPhoto         = "Добавить фото"
	ActionSearch           = "Поиск"
	ActionFilter           = "Фильтры"
	ActionMarkDone         = "Отметить выполненным"
	ActionLogIn            = "Войти"
	ActionAddUser          = "Новый пользователь"
	ActionAddSeason        = "Новый сезон"
	ActionAddDrop          = "Новый дроп"
	ActionAddModel         = "Новая модель"
	ActionAddMilestoneType = "Новый этап"
	ActionAddTemplate      = "Новый критический путь"
	ActionAddTemplateItem  = "Добавить этап"
	ActionRemove           = "Удалить"
	ActionMoveUp           = "Выше"
	ActionMoveDown         = "Ниже"
	ActionPreview          = "Показать даты"
	ActionResetPasswd      = "Сбросить пароль"
	ActionBackToList       = "Назад к списку"
	ActionClose            = "Закрыть"
	ActionEdit             = "Изменить"
)

// Common  strings that are used across ddifferent screens.
const (
	Saved = "Изменения сохранены."

	StateColumn   = "Состояние"
	StateActive   = "Активен"
	StateInactive = "Неактивен"

	// The choice a filter offers for "do not filter on this at all".
	FilterAny = "Все"

	// What stands between a name and the value that narrows it: "D2 · 01.10.2026".
	Separator = " · "
)

// The audit trail every card carries: the heading, the columns, and the words
// the actions read as on more than one screen.
const (
	AuditTitle = "История изменений"
	AuditEmpty = "Изменений пока нет."
	AuditWhen  = "Когда"
	AuditWho   = "Кто"
	AuditWhat  = "Что изменилось"

	AuditName = "Название изменено"
	AuditOff  = "Запись отключена"
	AuditOn   = "Запись включена"

	// An action recorded and not named anywhere.
	AuditOther = "Изменение"
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

// The words the audit trail on the user card reads as.
const (
	UsersAuditCreated   = "Пользователь создан"
	UsersAuditFirstName = "Имя изменено"
	UsersAuditLastName  = "Фамилия изменена"
	UsersAuditRole      = "Роль изменена"
	UsersAuditOff       = "Доступ отключён"
	UsersAuditOn        = "Доступ включён"
	UsersAuditPasswd    = "Пароль изменён"
	UsersAuditReset     = "Пароль сброшен"
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

	SeasonsAuditCreated = "Сезон создан"
	SeasonsAuditStart   = "Начало сезона изменено"
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
	DropsSeasonFixed = "Сезон дропа."

	DropsAuditCreated = "Дроп создан"
	DropsAuditTarget  = "Плановая дата изменена"
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
	ModelsFilterState = "Состояние"

	// The dialog that edits the header of the card.
	ModelsHeaderHint = "Артикул и состояние. Сезон и дроп у модели не меняются."

	// The card that says which step the model comes to next. %s of ModelsNextIn
	// is a count of days.
	ModelsNext          = "Ближайший этап"
	ModelsNextAllClosed = "Все этапы закрыты"
	ModelsNextLate      = "просрочен"
	ModelsNextToday     = "сегодня"
	ModelsNextIn        = "через %s"

	// The critical path control of the create screen. It builds the calendar of
	// the new model, and it offers the choice that builds none.
	ModelsTemplateHint = "Этапы будут рассчитаны от плановой даты дропа."
	ModelsNoCalendar   = "Без критического пути"
)

// The photo strip of the model card.
const (
	ModelsPhotos      = "Фотографии"
	ModelsPhotosEmpty = "Фотографий пока нет."
	ModelsPhotoCover  = "Обложка"
	ModelsPhotoOf     = "Фото модели"
	ModelsPhotoUpload = "Выберите изображения"

	ActionManagePhotos    = "Изменить"
	ModelsPhotosCoverHint = "Верхнее фото будет обложкой модели."

	ActionPhotoRemove = "Удалить"

	// What a refused upload or a refused reorder reports.
	ModelsPhotoNone     = "Выберите хотя бы один файл."
	ModelsPhotoTooLarge = "Файл слишком большой."
	ModelsPhotoRejected = "Такой файл загрузить нельзя. Подойдёт JPEG, PNG, WebP или GIF."
	ModelsPhotoOrder    = "Порядок фотографий устарел. Откройте страницу заново."
)

const (
	ModelsAuditCreated      = "Модель создана"
	ModelsAuditArticle      = "Артикул изменён"
	ModelsAuditPhotoAdded   = "Фото добавлено"
	ModelsAuditPhotoRemoved = "Фото удалено"
	ModelsAuditPhotoReorder = "Порядок фотографий изменён"
	ModelsAuditTemplate     = "Критический путь применён"
)

// The milestone types section
const (
	MilestoneTypesTitle     = "Этапы"
	MilestoneTypesNewTitle  = "Новый этап"
	MilestoneTypesEditTitle = "Карточка этапа"
	MilestoneTypesEmpty     = "Этапов пока нет."

	MilestoneTypesName            = "Название"
	MilestoneTypesNameHint        = "Например, «Отшив»."
	MilestoneTypesDescription     = "Описание"
	MilestoneTypesDescriptionHint = "Краткое описание этапа."

	MilestoneTypesAuditCreated     = "Этап создан"
	MilestoneTypesAuditDescription = "Описание изменено"
)

// The milestone templates section: the list, the create screen, and the editor
// of one template.
const (
	MilestoneTemplatesTitle    = "Критические пути"
	MilestoneTemplatesNewTitle = "Новый критический путь"
	MilestoneTemplatesEmpty    = "Шаблонов пока нет."

	MilestoneTemplatesName            = "Название"
	MilestoneTemplatesNameHint        = "Например, «Импорт, ж/д»."
	MilestoneTemplatesDescription     = "Описание"
	MilestoneTemplatesDescriptionHint = "Для каких моделей этот путь."
	MilestoneTemplatesDefault         = "Путь по умолчанию"
	MilestoneTemplatesDefaultHint     = "Автоматически добавлять этот путь при создании новой модели."
	MilestoneTemplatesDefaultMark     = "По умолчанию"

	// The item table of the editor.
	MilestoneItemsTitle    = "Этапы пути"
	MilestoneItemsEmpty    = "В этом пути пока нет этапов."
	MilestoneItemsStep     = "Добавить этап"
	MilestoneItemsOffset   = "Сдвиг, дней"
	MilestoneItemsGap      = "Интервал, дней"
	MilestoneItemsDate     = "Дата"
	MilestoneItemsOrder    = "Порядок"
	MilestoneItemsHint     = "Сдвиг — дней до плановой даты дропа, ноль или меньше. Интервал — дней от предыдущего этапа."
	MilestoneItemsNoType   = "Этот путь уже содержит все существующие этапы."
	MilestoneItemsTypeHint = "Этап добавляется в конец списка."
	MilestoneItemsStale    = "Порядок этапов устарел. Откройте страницу заново."

	// The preview beside the table: a target date, and the dates the path
	// produces from it.
	MilestonePreviewTarget = "Плановая дата дропа"
	MilestonePreviewEmpty  = "Укажите плановую дату дропа, чтобы увидеть даты этапов."

	MilestoneTemplatesAuditCreated    = "Критический путь создан"
	MilestoneTemplatesAuditDefaultOn  = "Путь назначен по умолчанию"
	MilestoneTemplatesAuditDefaultOff = "Путь больше не по умолчанию"
	MilestoneTemplatesAuditOffset     = "Сдвиг этапа изменён"
	MilestoneTemplatesAuditAdded      = "Этап добавлен"
	MilestoneTemplatesAuditRemoved    = "Этап удалён"
	MilestoneTemplatesAuditReorder    = "Порядок этапов изменён"
)

// The calendar section of the model screen.
const (
	MilestonesTitle = "Критический путь"
	MilestonesHint  = "Этапы модели в порядке плановых дат. Нажмите этап, чтобы увидеть базу и комментарий."
	MilestonesEmpty = "У модели пока нет критического пути."

	MilestonesStep     = "Этап"
	MilestonesBaseline = "База"
	MilestonesPlan     = "План"
	MilestonesFact     = "Факт"
	MilestonesState    = "Статус"
	MilestonesSlip     = "Отклонение"

	MilestonesStateDone    = "Готово"
	MilestonesStateLate    = "Просрочен"
	MilestonesStateDue     = "Скоро"
	MilestonesStatePlanned = "В плане"

	MilestonesOnTime  = "В срок"
	MilestonesNote    = "Комментарий"
	MilestonesRetired = "Снятые этапы"

	// What one row says beside its dates. %s of MilestonesLateBy is a count of
	// days, and so is %s of MilestonesSlipBy.
	MilestonesLateBy     = "на %s"
	MilestonesHasNote    = "Есть комментарий"
	MilestonesLastChange = "Последнее изменение"

	// What the card reports after a write the row made by itself. %s of
	// MilestonesFactNotice names the step, and %s the day it was stamped with.
	MilestonesFactNotice    = "Факт проставлен: %s — %s."
	MilestonesRetiredNotice = "Этап снят."

	// The two controls of the section: apply a critical path, and add one step
	// the model does not hold.
	MilestonesTemplate     = "Критический путь"
	MilestonesTemplateHint = "Добавляет этапы, которых у модели ещё нет. Даты уже стоящих этапов не меняются."
	MilestonesNoTemplate   = "Активных критических путей пока нет."
	MilestonesAddStep      = "Добавить этап"
	MilestonesAddStepHint  = "Этап вне критического пути. Укажите плановую дату."
	MilestonesNoType       = "У модели уже есть все существующие этапы."
	MilestonesStale        = "Этап уже изменён. Откройте страницу заново."

	// The dialog that edits one step, and the dialog that builds the calendar.
	MilestonesEditHint      = "План, факт и комментарий этапа."
	MilestonesPlanHint      = "База %s. Перенос сдвигает следующие этапы."
	MilestonesFactHint      = "Дата в будущем недоступна."
	MilestonesCalendarTitle = "Этапы и критический путь"
	MilestonesCalendarHint  = "Применить путь целиком или добавить один этап."

	// What the dialog shows before it moves a plan date: how far the move goes,
	// and the switch that keeps the steps after it where they stand. %s of the
	// two shift lines is a count of days.
	MilestonesShiftLater   = "Перенос на %s позже"
	MilestonesShiftEarlier = "Перенос на %s раньше"
	MilestonesMoveShift    = "Сдвинуть следующие этапы"
)

// The milestone list: the late work of a whole season on one screen. It is the
// one milestone screen a member reaches.
const (
	MilestoneListTitle    = "Отставания"
	MilestoneListHint     = "Этапы сезона: сначала те, что отстают сильнее."
	MilestoneListEmpty    = "В этом сезоне пока нет этапов."
	MilestoneListNoMatch  = "По этим фильтрам ничего не найдено."
	MilestoneListNoSeason = "Сначала создайте сезон: список этапов существует внутри сезона."

	// The controls above the list. The other three reuse the words of the model
	// list and of the calendar section.
	MilestoneListStateOpen  = "Просроченные и ближайшие"
	MilestoneListSearch     = "Поиск по артикулу"
	MilestoneListSearchHint = "Артикул"
	MilestoneListGroup      = "Группировка"
	MilestoneListGroupNone  = "Без группировки"
	MilestoneListGroupDrop  = "По дропу"
	MilestoneListGroupType  = "По этапу"

	// The models of the season that hold no critical path. Such a model never
	// reads as late, so this is the only place it appears.
	MilestoneListNoCalendar     = "Модели без критического пути"
	MilestoneListNoCalendarHint = "У этих моделей нет этапов, поэтому в списке их не видно."
)

// The actions of the calendar section.
const (
	ActionApplyTemplate = "Применить путь"
	ActionAddMilestone  = "Добавить"
	ActionFactDone      = "Выполнено"
	ActionFactToday     = "Сегодня"
	ActionRetireStep    = "Снять этап"
	ActionRestoreStep   = "Вернуть этап"
)

// The calendar of one model: what the trail of a milestone reads as.
const (
	MilestonesAuditCreated     = "Этап добавлен"
	MilestonesAuditPlan        = "План перенесён"
	MilestonesAuditFact        = "Факт проставлен"
	MilestonesAuditFactMoved   = "Факт изменён"
	MilestonesAuditFactCleared = "Факт снят"
	MilestonesAuditNote        = "Комментарий изменён"
)

// The failures in the http middleware chain
const (
	ErrorUnexpected = "Что-то пошло не так. Попробуйте ещё раз."
	ErrorForbidden  = "Запрос отклонён."
)
