package telegram

import (
	"fmt"

	"github.com/zelenin/go-tdlib/client"
)

// clientAdapter — внутренний интерфейс-обёртка над *client.Client.
// Содержит только реально используемые методы go-tdlib, а не полную поверхность.
// Сигнатуры 1-в-1 с go-tdlib (без ctx и без domain-типов).
//
// Метод добавляется сюда только когда у него появился живой потребитель в repo.go
// или в telegramRepo-интерфейсах сервисов. Статические пакетные функции go-tdlib
// (client.ParseTextEntities, client.GetMarkdownText) не являются методами
// *client.Client и в интерфейс не входят.
type clientAdapter interface {
	// Операции с сообщениями.
	SendMessage(*client.SendMessageRequest) (*client.Message, error)
	SendMessageAlbum(*client.SendMessageAlbumRequest) (*client.Messages, error)
	ForwardMessages(*client.ForwardMessagesRequest) (*client.Messages, error)
	GetMessage(*client.GetMessageRequest) (*client.Message, error)
	GetMessages(*client.GetMessagesRequest) (*client.Messages, error)
	EditMessageText(*client.EditMessageTextRequest) (*client.Message, error)
	EditMessageCaption(*client.EditMessageCaptionRequest) (*client.Message, error)
	DeleteMessages(*client.DeleteMessagesRequest) (*client.Ok, error)

	// Операции со ссылками.
	GetMessageLink(*client.GetMessageLinkRequest) (*client.MessageLink, error)
	GetMessageLinkInfo(*client.GetMessageLinkInfoRequest) (*client.MessageLinkInfo, error)

	// Текст и callback.
	TranslateText(*client.TranslateTextRequest) (*client.FormattedText, error)
	GetCallbackQueryAnswer(*client.GetCallbackQueryAnswerRequest) (*client.CallbackQueryAnswer, error)

	// Чаты.
	LoadChats(*client.LoadChatsRequest) (*client.Ok, error)
	GetChatHistory(*client.GetChatHistoryRequest) (*client.Messages, error)
	GetChat(*client.GetChatRequest) (*client.Chat, error)

	// Управление чатами (для cmd/stand).
	CreateNewSupergroupChat(*client.CreateNewSupergroupChatRequest) (*client.Chat, error)
	CreateNewBasicGroupChat(*client.CreateNewBasicGroupChatRequest) (*client.CreatedBasicGroupChat, error)
	SetSupergroupUsername(*client.SetSupergroupUsernameRequest) (*client.Ok, error)
	DeleteChat(*client.DeleteChatRequest) (*client.Ok, error)

	// Системные операции и авторизация.
	GetMe() (*client.User, error)
	LogOut() (*client.Ok, error)
	GetListener() *client.Listener
}

// Статическая проверка, что *client.Client и *Repo реализуют clientAdapter.
var (
	_ clientAdapter = (*client.Client)(nil)
	_ clientAdapter = (*Repo)(nil)
)

// --- Операции с сообщениями ---

// SendMessage отправляет сообщение.
func (r *Repo) SendMessage(req *client.SendMessageRequest) (*client.Message, error) {
	msg, err := r.clientAdapter.SendMessage(req)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}
	return msg, nil
}

// SendMessageAlbum отправляет медиа-альбом.
func (r *Repo) SendMessageAlbum(req *client.SendMessageAlbumRequest) (*client.Messages, error) {
	msgs, err := r.clientAdapter.SendMessageAlbum(req)
	if err != nil {
		return nil, fmt.Errorf("send message album: %w", err)
	}
	return msgs, nil
}

// ForwardMessages пересылает сообщения.
func (r *Repo) ForwardMessages(req *client.ForwardMessagesRequest) (*client.Messages, error) {
	msgs, err := r.clientAdapter.ForwardMessages(req)
	if err != nil {
		return nil, fmt.Errorf("forward messages: %w", err)
	}
	return msgs, nil
}

// GetMessage возвращает сообщение.
func (r *Repo) GetMessage(req *client.GetMessageRequest) (*client.Message, error) {
	msg, err := r.clientAdapter.GetMessage(req)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	return msg, nil
}

// GetMessages возвращает сообщения batch-ом.
func (r *Repo) GetMessages(req *client.GetMessagesRequest) (*client.Messages, error) {
	msgs, err := r.clientAdapter.GetMessages(req)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	return msgs, nil
}

// EditMessageText редактирует текст сообщения.
func (r *Repo) EditMessageText(req *client.EditMessageTextRequest) (*client.Message, error) {
	msg, err := r.clientAdapter.EditMessageText(req)
	if err != nil {
		return nil, fmt.Errorf("edit message text: %w", err)
	}
	return msg, nil
}

// EditMessageCaption редактирует подпись медиа-сообщения.
func (r *Repo) EditMessageCaption(req *client.EditMessageCaptionRequest) (*client.Message, error) {
	msg, err := r.clientAdapter.EditMessageCaption(req)
	if err != nil {
		return nil, fmt.Errorf("edit message caption: %w", err)
	}
	return msg, nil
}

// DeleteMessages удаляет сообщения.
func (r *Repo) DeleteMessages(req *client.DeleteMessagesRequest) (*client.Ok, error) {
	ok, err := r.clientAdapter.DeleteMessages(req)
	if err != nil {
		return nil, fmt.Errorf("delete messages: %w", err)
	}
	return ok, nil
}

// --- Операции со ссылками ---

// GetMessageLink возвращает публичную ссылку на сообщение.
func (r *Repo) GetMessageLink(req *client.GetMessageLinkRequest) (*client.MessageLink, error) {
	link, err := r.clientAdapter.GetMessageLink(req)
	if err != nil {
		return nil, fmt.Errorf("get message link: %w", err)
	}
	return link, nil
}

// GetMessageLinkInfo парсит ссылку и возвращает информацию о сообщении.
func (r *Repo) GetMessageLinkInfo(req *client.GetMessageLinkInfoRequest) (*client.MessageLinkInfo, error) {
	info, err := r.clientAdapter.GetMessageLinkInfo(req)
	if err != nil {
		return nil, fmt.Errorf("get message link info: %w", err)
	}
	return info, nil
}

// --- Текст и callback ---

// TranslateText переводит текст.
func (r *Repo) TranslateText(req *client.TranslateTextRequest) (*client.FormattedText, error) {
	result, err := r.clientAdapter.TranslateText(req)
	if err != nil {
		return nil, fmt.Errorf("translate text: %w", err)
	}
	return result, nil
}

// GetCallbackQueryAnswer получает ответ на callback-запрос.
func (r *Repo) GetCallbackQueryAnswer(req *client.GetCallbackQueryAnswerRequest) (*client.CallbackQueryAnswer, error) {
	answer, err := r.clientAdapter.GetCallbackQueryAnswer(req)
	if err != nil {
		return nil, fmt.Errorf("get callback query answer: %w", err)
	}
	return answer, nil
}

// --- Чаты ---

// LoadChats загружает список чатов.
func (r *Repo) LoadChats(req *client.LoadChatsRequest) (*client.Ok, error) {
	ok, err := r.clientAdapter.LoadChats(req)
	if err != nil {
		return nil, fmt.Errorf("load chats: %w", err)
	}
	return ok, nil
}

// GetChatHistory возвращает историю сообщений чата.
func (r *Repo) GetChatHistory(req *client.GetChatHistoryRequest) (*client.Messages, error) {
	msgs, err := r.clientAdapter.GetChatHistory(req)
	if err != nil {
		return nil, fmt.Errorf("get chat history: %w", err)
	}
	return msgs, nil
}

// GetChat возвращает информацию о чате.
func (r *Repo) GetChat(req *client.GetChatRequest) (*client.Chat, error) {
	chat, err := r.clientAdapter.GetChat(req)
	if err != nil {
		return nil, fmt.Errorf("get chat: %w", err)
	}
	return chat, nil
}

// --- Управление чатами (stand) ---

// CreateNewSupergroupChat создаёт супергруппу или канал.
func (r *Repo) CreateNewSupergroupChat(req *client.CreateNewSupergroupChatRequest) (*client.Chat, error) {
	chat, err := r.clientAdapter.CreateNewSupergroupChat(req)
	if err != nil {
		return nil, fmt.Errorf("create supergroup chat: %w", err)
	}
	return chat, nil
}

// CreateNewBasicGroupChat создаёт базовую группу.
func (r *Repo) CreateNewBasicGroupChat(req *client.CreateNewBasicGroupChatRequest) (*client.CreatedBasicGroupChat, error) {
	chat, err := r.clientAdapter.CreateNewBasicGroupChat(req)
	if err != nil {
		return nil, fmt.Errorf("create basic group chat: %w", err)
	}
	return chat, nil
}

// SetSupergroupUsername устанавливает username супергруппы или канала.
func (r *Repo) SetSupergroupUsername(req *client.SetSupergroupUsernameRequest) (*client.Ok, error) {
	ok, err := r.clientAdapter.SetSupergroupUsername(req)
	if err != nil {
		return nil, fmt.Errorf("set supergroup username: %w", err)
	}
	return ok, nil
}

// DeleteChat удаляет чат.
func (r *Repo) DeleteChat(req *client.DeleteChatRequest) (*client.Ok, error) {
	ok, err := r.clientAdapter.DeleteChat(req)
	if err != nil {
		return nil, fmt.Errorf("delete chat: %w", err)
	}
	return ok, nil
}

// --- Системные операции ---

// GetMe возвращает информацию о текущем пользователе.
func (r *Repo) GetMe() (*client.User, error) {
	user, err := r.clientAdapter.GetMe()
	if err != nil {
		return nil, fmt.Errorf("get me: %w", err)
	}
	return user, nil
}

// LogOut завершает сессию TDLib (тонкая обёртка; полный composite — в repo.go).
func (r *Repo) LogOut() (*client.Ok, error) {
	ok, err := r.clientAdapter.LogOut()
	if err != nil {
		return nil, fmt.Errorf("log out: %w", err)
	}
	return ok, nil
}

// GetListener возвращает listener TDLib для подписки на обновления.
func (r *Repo) GetListener() *client.Listener {
	return r.clientAdapter.GetListener()
}

// --- Статические функции ---

// GetOption возвращает значение опции TDLib. Тонкая обёртка над пакетной функцией
// client.GetOption (не является методом *client.Client; доступна до авторизации).
func (r *Repo) GetOption(req *client.GetOptionRequest) (client.OptionValue, error) {
	return client.GetOption(req)
}
