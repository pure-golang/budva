package domain

// UpdateType определяет тип обновления от Telegram.
type UpdateType int

const (
	// UpdateNewMessage — новое сообщение.
	UpdateNewMessage UpdateType = iota
	// UpdateMessageEdited — сообщение отредактировано.
	UpdateMessageEdited
	// UpdateDeleteMessages — сообщения удалены.
	UpdateDeleteMessages
	// UpdateMessageSendSucceeded — сообщение успешно отправлено (temp→real ID).
	UpdateMessageSendSucceeded
)

// Update описывает обновление от Telegram.
type Update struct {
	// Type — тип обновления.
	Type UpdateType

	// Поля для UpdateNewMessage и UpdateMessageEdited:
	Message *Message

	// Поля для UpdateDeleteMessages:
	ChatID      ChatID
	MessageIDs  []MessageID
	IsPermanent bool

	// Поля для UpdateMessageSendSucceeded:
	OldMessageID MessageID
}

// Status описывает текущий статус подключения к Telegram.
type Status struct {
	// TDLibVersion — версия библиотеки TDLib.
	TDLibVersion string
	// UserID — идентификатор авторизованного пользователя.
	UserID int64
}

// MessageLinkInfo описывает информацию о ссылке на сообщение.
type MessageLinkInfo struct {
	// ChatID — чат, в котором находится сообщение.
	ChatID ChatID
	// MessageID — идентификатор сообщения.
	MessageID MessageID
}
