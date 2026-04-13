package domain

// Source описывает настройки источника сообщений.
type Source struct {
	// ChatID — идентификатор чата-источника, заполняется при загрузке.
	ChatID ChatID
	// Translate — настройки перевода сообщений.
	Translate *Translate
	// Sign — настройки подписи для сообщений из этого источника.
	Sign *Sign
	// Link — настройки ссылки на оригинал сообщения.
	Link *Link
	// AutoAnswer — включить автоматические ответы на callback-запросы.
	AutoAnswer bool
	// DeleteSystemMessages — удалять системные сообщения из источника.
	DeleteSystemMessages bool
	// Prev — настройки ссылки на предыдущую версию сообщения.
	Prev *Prev
	// Next — настройки ссылки на следующую версию сообщения.
	Next *Next
}

// Translate описывает настройки перевода сообщений.
type Translate struct {
	// Lang — целевой язык перевода.
	Lang string
	// For — список чатов, для которых применяется перевод.
	For []ChatID
}

// Sign описывает настройки подписи источника.
type Sign struct {
	// Title — текст подписи с поддержкой разметки.
	Title string
	// For — список чатов, для которых применяется подпись.
	For []ChatID
}

// Link описывает настройки ссылки на оригинал.
type Link struct {
	// Title — текст ссылки с поддержкой разметки.
	Title string
	// For — список чатов, для которых применяется ссылка.
	For []ChatID
}

// Prev описывает настройки ссылки на предыдущую версию.
type Prev struct {
	// Title — текст ссылки с поддержкой разметки.
	Title string
	// For — список чатов, для которых применяется ссылка.
	For []ChatID
}

// Next описывает настройки ссылки на следующую версию.
type Next struct {
	// Title — текст ссылки с поддержкой разметки.
	Title string
	// For — список чатов, для которых применяется ссылка.
	For []ChatID
}

const (
	// SignTitle — заголовок подписи по умолчанию.
	SignTitle = "Sign"
	// LinkTitle — заголовок ссылки на источник по умолчанию.
	LinkTitle = "🔗Link"
	// PrevTitle — заголовок ссылки на предыдущую версию по умолчанию.
	PrevTitle = "↖Prev"
	// NextTitle — заголовок ссылки на следующую версию по умолчанию.
	NextTitle = "↘Next"
)
