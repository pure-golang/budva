package domain

// Message описывает сообщение Telegram.
type Message struct {
	// ChatID — идентификатор чата.
	ChatID ChatID
	// ID — идентификатор сообщения.
	ID MessageID
	// MediaAlbumID — идентификатор медиа-группы (0 если не альбом).
	MediaAlbumID int64
	// Content — содержимое сообщения.
	Content MessageContent
	// ForwardInfo — информация о пересылке (nil если не переслано).
	ForwardInfo *MessageForwardInfo
	// ReplyTo — информация об ответе (nil если не ответ).
	ReplyTo *MessageReplyTo
	// CanBeSaved — можно ли сохранить/переслать сообщение.
	CanBeSaved bool
	// ReplyMarkup — inline-клавиатура (nil если нет).
	ReplyMarkup *ReplyMarkup
}

// FormattedText описывает текст с форматированием.
type FormattedText struct {
	// Text — текстовое содержимое.
	Text string
	// Entities — список форматирующих сущностей.
	Entities []TextEntity
}

// DeepCopy создаёт глубокую копию FormattedText.
func (ft *FormattedText) DeepCopy() *FormattedText {
	if ft == nil {
		return nil
	}
	cp := &FormattedText{Text: ft.Text}
	if len(ft.Entities) > 0 {
		cp.Entities = make([]TextEntity, len(ft.Entities))
		copy(cp.Entities, ft.Entities)
	}
	return cp
}

// TextEntity описывает форматирующую сущность в тексте.
type TextEntity struct {
	// Offset — смещение в UTF-16 единицах.
	Offset int32
	// Length — длина в UTF-16 единицах.
	Length int32
	// Type — тип форматирования.
	Type TextEntityType
	// URL — URL для TextEntityTextURL.
	URL string
}

// TextEntityType определяет тип текстовой сущности.
type TextEntityType int

const (
	// TextEntityPlain — обычный текст (не форматирован).
	TextEntityPlain TextEntityType = iota
	// TextEntityURL — ссылка в тексте.
	TextEntityURL
	// TextEntityTextURL — текст со скрытой ссылкой.
	TextEntityTextURL
	// TextEntityBold — жирный текст.
	TextEntityBold
	// TextEntityItalic — курсив.
	TextEntityItalic
	// TextEntityStrikethrough — зачёркнутый текст.
	TextEntityStrikethrough
	// TextEntityCode — моноширинный текст.
	TextEntityCode
)

// MessageContent описывает содержимое сообщения.
type MessageContent struct {
	// Type — тип контента.
	Type MessageContentType
	// Text — форматированный текст (для MessageText) или подпись (для медиа).
	Text *FormattedText
	// FileID — удалённый идентификатор файла (для медиа-контента).
	FileID string
	// ThumbnailFileID — удалённый идентификатор миниатюры.
	ThumbnailFileID string
	// Width — ширина (фото/видео/анимация).
	Width int32
	// Height — высота (фото/видео/анимация).
	Height int32
	// Duration — длительность (видео/аудио/голосовое сообщение).
	Duration int32
	// FileName — имя файла (документ/аудио).
	FileName string
	// MimeType — MIME-тип.
	MimeType string
	// DisableLinkPreview — отключить предпросмотр ссылок (для MessageText).
	DisableLinkPreview bool
}

// MessageContentType определяет тип контента сообщения.
type MessageContentType int

const (
	// ContentText — текстовое сообщение.
	ContentText MessageContentType = iota
	// ContentPhoto — фотография.
	ContentPhoto
	// ContentVideo — видео.
	ContentVideo
	// ContentDocument — документ.
	ContentDocument
	// ContentAudio — аудиофайл.
	ContentAudio
	// ContentAnimation — анимация (GIF).
	ContentAnimation
	// ContentVoiceNote — голосовое сообщение.
	ContentVoiceNote
	// ContentSystem — системное сообщение (добавление/удаление участников и т.д.).
	ContentSystem
	// ContentUnknown — неизвестный тип контента.
	ContentUnknown
)

// AlbumItem описывает один элемент альбома для отправки через фасад.
type AlbumItem struct {
	// Text — текст или подпись.
	Text string
	// FilePath — локальный путь к файлу (пустая строка для текстовых сообщений).
	FilePath string
}

// ContentTypeByFileExt определяет тип контента по расширению файла.
func ContentTypeByFileExt(ext string) MessageContentType {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return ContentPhoto
	case ".mp4", ".mov", ".avi", ".mkv", ".webm":
		return ContentVideo
	case ".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac", ".wma", ".opus":
		return ContentAudio
	default:
		return ContentDocument
	}
}

// IsMediaType возвращает true для медиа-типов контента.
func (t MessageContentType) IsMediaType() bool {
	switch t {
	case ContentPhoto, ContentVideo, ContentDocument, ContentAudio, ContentAnimation, ContentVoiceNote:
		return true
	}
	return false
}

// HasCaption возвращает true если тип контента может иметь подпись.
func (t MessageContentType) HasCaption() bool {
	return t.IsMediaType()
}

// MessageForwardInfo описывает информацию о пересылке.
type MessageForwardInfo struct {
	// OriginChatID — исходный чат при пересылке из канала (0 если не канал).
	OriginChatID ChatID
	// OriginMessageID — ID оригинального сообщения при пересылке из канала.
	OriginMessageID MessageID
}

// MessageReplyTo описывает информацию об ответе на сообщение.
type MessageReplyTo struct {
	// ChatID — чат с оригинальным сообщением.
	ChatID ChatID
	// MessageID — ID оригинального сообщения.
	MessageID MessageID
}

// ReplyMarkup описывает inline-клавиатуру сообщения.
type ReplyMarkup struct {
	// CallbackData — данные callback-кнопки (первая кнопка первого ряда).
	CallbackData []byte
}

// InputMessageContent описывает контент для отправки нового сообщения.
type InputMessageContent struct {
	// Type — тип контента.
	Type MessageContentType
	// Text — форматированный текст или подпись.
	Text *FormattedText
	// FileID — удалённый идентификатор файла.
	FileID string
	// ThumbnailFileID — удалённый идентификатор миниатюры.
	ThumbnailFileID string
	// Width — ширина.
	Width int32
	// Height — высота.
	Height int32
	// Duration — длительность.
	Duration int32
	// FileName — имя файла.
	FileName string
	// MimeType — MIME-тип.
	MimeType string
	// FilePath — локальный путь к файлу для загрузки.
	FilePath string
	// DisableLinkPreview — отключить предпросмотр ссылок.
	DisableLinkPreview bool
	// ReplyToMessageID — ID сообщения, на которое отвечаем.
	ReplyToMessageID MessageID
}
