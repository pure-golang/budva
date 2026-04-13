package domain

// Source описывает настройки источника сообщений.
type Source struct {
	// ChatID — идентификатор чата-источника, заполняется при загрузке.
	ChatID ChatID `yaml:"-"`
	// Translate — настройки перевода сообщений.
	Translate *Translate `yaml:"translate"`
	// Sign — настройки подписи для сообщений из этого источника.
	Sign *Sign `yaml:"sign"`
	// Link — настройки ссылки на оригинал сообщения.
	Link *Link `yaml:"link"`
	// AutoAnswer — включить автоматические ответы на callback-запросы.
	AutoAnswer bool `yaml:"autoAnswer"`
	// DeleteSystemMessages — удалять системные сообщения из источника.
	DeleteSystemMessages bool `yaml:"deleteSystemMessages"`
	// Prev — настройки ссылки на предыдущую версию сообщения.
	Prev *Prev `yaml:"prev"`
	// Next — настройки ссылки на следующую версию сообщения.
	Next *Next `yaml:"next"`
}

// Translate описывает настройки перевода сообщений.
type Translate struct {
	// Lang — целевой язык перевода.
	Lang string `yaml:"lang"`
	// For — список чатов, для которых применяется перевод.
	For []ChatID `yaml:"for"`
}

// Sign описывает настройки подписи источника.
type Sign struct {
	// Title — текст подписи с поддержкой разметки.
	Title string `yaml:"title"`
	// For — список чатов, для которых применяется подпись.
	For []ChatID `yaml:"for"`
}

// Link описывает настройки ссылки на оригинал.
type Link struct {
	// Title — текст ссылки с поддержкой разметки.
	Title string `yaml:"title"`
	// For — список чатов, для которых применяется ссылка.
	For []ChatID `yaml:"for"`
}

// Prev описывает настройки ссылки на предыдущую версию.
type Prev struct {
	// Title — текст ссылки с поддержкой разметки.
	Title string `yaml:"title"`
	// For — список чатов, для которых применяется ссылка.
	For []ChatID `yaml:"for"`
}

// Next описывает настройки ссылки на следующую версию.
type Next struct {
	// Title — текст ссылки с поддержкой разметки.
	Title string `yaml:"title"`
	// For — список чатов, для которых применяется ссылка.
	For []ChatID `yaml:"for"`
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
