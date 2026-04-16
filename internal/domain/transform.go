package domain

// TransformParams описывает параметры трансформации текста сообщения.
type TransformParams struct {
	// Text — форматированный текст для трансформации.
	Text *FormattedText
	// Source — настройки источника.
	Source *Source
	// Destination — настройки получателя.
	Destination *Destination
	// DstChatID — идентификатор целевого чата.
	DstChatID ChatID
	// SrcChatID — идентификатор исходного чата.
	SrcChatID ChatID
	// SrcMessageID — идентификатор исходного сообщения.
	SrcMessageID MessageID
	// PrevMessageID — идентификатор предыдущей версии сообщения (0 если нет).
	PrevMessageID MessageID
	// WithSources — добавлять подпись и ссылку на источник.
	WithSources bool
	// ForAlbum — сообщение является частью альбома (влияет на формат ссылки).
	ForAlbum bool
	// ReplyMarkup — данные callback-кнопки.
	ReplyMarkup []byte
}
