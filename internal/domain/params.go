package domain

import "github.com/zelenin/go-tdlib/client"

// TransformParams описывает параметры одного вызова Transform.
//
// Text приходит в формате TDLib (`*client.FormattedText`) — тот же контракт,
// что и в `Message.Content.*.Text/Caption`.
type TransformParams struct {
	// Text — форматированный текст для трансформации.
	Text *client.FormattedText
	// Source — настройки источника.
	Source *Source
	// Destination — настройки получателя.
	Destination *Destination
	// DstChatID — идентификатор целевого чата.
	DstChatID int64
	// SrcChatID — идентификатор исходного чата.
	SrcChatID int64
	// SrcMessageID — идентификатор исходного сообщения.
	SrcMessageID int64
	// PrevMessageID — идентификатор предыдущей версии сообщения (0 если нет).
	PrevMessageID int64
	// WithSources — добавлять подпись и ссылку на источник.
	WithSources bool
	// ForAlbum — сообщение является частью альбома (влияет на формат ссылки).
	ForAlbum bool
	// ReplyMarkup — данные callback-кнопки.
	ReplyMarkup []byte
}

// AlbumItem — элемент медиа-альбома для входящей команды SendMessageAlbum.
type AlbumItem struct {
	Text     string
	FilePath string
}
