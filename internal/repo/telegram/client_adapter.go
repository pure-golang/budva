package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// sendMessageTimeout — максимальное время ожидания permanent ID после SendMessage.
const sendMessageTimeout = 30 * time.Second

// clientAdapter — контракт TDLib-клиента.
type clientAdapter interface {
	// Операции с сообщениями.
	SendMessage(ctx context.Context, chatID domain.ChatID, content domain.InputMessageContent) (domain.MessageID, error)
	SendMessageAlbum(ctx context.Context, chatID domain.ChatID, contents []domain.InputMessageContent) ([]domain.MessageID, error)
	ForwardMessages(ctx context.Context, fromChatID, toChatID domain.ChatID, messageIDs []domain.MessageID) ([]domain.MessageID, error)
	GetMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error)
	EditMessageText(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error
	EditMessageCaption(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error
	DeleteMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID, revoke bool) error

	// Операции со ссылками.
	GetMessageLink(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, forAlbum bool) (string, error)
	GetMessageLinkInfo(ctx context.Context, url string) (*domain.MessageLinkInfo, error)

	// Операции с текстом.
	TranslateText(ctx context.Context, text *domain.FormattedText, lang string) (*domain.FormattedText, error)
	GetCallbackQueryAnswer(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, payload []byte) (string, error)
	ParseTextEntities(ctx context.Context, text string) (*domain.FormattedText, error)

	// Операции с чатами.
	LoadChats(ctx context.Context, limit int32) error
	WarmUpChat(ctx context.Context, chatID domain.ChatID, limit int32) error
	GetChatHistory(ctx context.Context, chatID domain.ChatID, fromMessageID domain.MessageID, offset int32, limit int32) ([]*domain.Message, error)
	GetChatType(ctx context.Context, chatID domain.ChatID) (string, error)

	// Операции с текстом (static TDLib methods).
	GetMarkdownText(ctx context.Context, text *domain.FormattedText) (*domain.FormattedText, error)

	// Системные операции.
	GetOption(ctx context.Context, name string) (string, error)
	GetMe(ctx context.Context) (int64, error)

	// Batch-операции.
	GetMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID) ([]*domain.Message, error)

	// Управление чатами (stand).
	CreateNewSupergroupChat(ctx context.Context, title string, isChannel bool, description string) (domain.ChatID, int64, error)
	CreateNewBasicGroupChat(ctx context.Context, title string, userIDs []int64) (domain.ChatID, error)
	SetSupergroupUsername(ctx context.Context, supergroupID int64, username string) error
	DeleteChat(ctx context.Context, chatID domain.ChatID) error

	// Отправка данных авторизации.
	SubmitPhone(ctx context.Context, phone string) error
	SubmitCode(ctx context.Context, code string) error
	SubmitPassword(ctx context.Context, password string) error
}

var _ clientAdapter = (*Repo)(nil)

// --- Авторизация ---

// LogOut завершает сессию TDLib.
func (r *Repo) LogOut(_ context.Context) error {
	if _, err := r.tdClient.LogOut(); err != nil {
		return fmt.Errorf("log out: %w", err)
	}
	r.tdClient = nil
	return nil
}

// CleanUp удаляет локальные данные TDLib (БД и файлы).
// После logout БД остаётся в состоянии LoggingOut, которое go-tdlib
// не умеет обрабатывать при следующем запуске. Удаление гарантирует
// чистый старт с WaitPhoneNumber.
func (r *Repo) CleanUp() {
	if r.cfg.DatabaseDirectory != "" {
		os.RemoveAll(r.cfg.DatabaseDirectory) //nolint:errcheck // Best-effort cleanup
	}
	if r.cfg.FilesDirectory != "" {
		os.RemoveAll(r.cfg.FilesDirectory) //nolint:errcheck // Best-effort cleanup
	}
}

// SubmitPhone отправляет номер телефона для авторизации.
func (r *Repo) SubmitPhone(_ context.Context, phone string) error {
	r.logger.Info("Phone submitted", slog.String("phone", domain.MaskPhoneNumber(phone)))
	r.mu.RLock()
	ch := r.phoneCh
	r.mu.RUnlock()
	ch <- phone
	return nil
}

// SubmitCode отправляет код подтверждения.
func (r *Repo) SubmitCode(_ context.Context, code string) error {
	r.logger.Info("Code submitted")
	r.mu.RLock()
	ch := r.codeCh
	r.mu.RUnlock()
	ch <- code
	return nil
}

// SubmitPassword отправляет пароль двухфакторной аутентификации.
func (r *Repo) SubmitPassword(_ context.Context, password string) error {
	r.logger.Info("Password submitted")
	r.mu.RLock()
	ch := r.passwordCh
	r.mu.RUnlock()
	ch <- password
	return nil
}

// --- Операции с сообщениями ---

// SendMessage отправляет сообщение в чат.
func (r *Repo) SendMessage(_ context.Context, chatID domain.ChatID, content domain.InputMessageContent) (domain.MessageID, error) {
	msg, err := r.tdClient.SendMessage(&client.SendMessageRequest{
		ChatId:              chatID,
		ReplyTo:             toTDLibInputMessageReplyTo(content.ReplyToMessageID),
		InputMessageContent: toTDLibInputMessageContent(content),
	})
	if err != nil {
		return 0, fmt.Errorf("send message: %w", err)
	}
	return msg.Id, nil
}

// SendMessageAndWait отправляет сообщение и ждёт присвоения permanent ID.
// TDLib возвращает temporary ID из SendMessage; permanent ID приходит
// асинхронно через UpdateMessageSendSucceeded. Метод блокирует до получения
// permanent ID или таймаута (30 сек).
func (r *Repo) SendMessageAndWait(ctx context.Context, chatID domain.ChatID, content domain.InputMessageContent) (*domain.Message, error) {
	// Создаём listener ДО отправки, чтобы не пропустить update
	listener := r.tdClient.GetListener()
	defer listener.Close()

	tmpID, err := r.SendMessage(ctx, chatID, content)
	if err != nil {
		return nil, err
	}

	timeout := time.After(sendMessageTimeout)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for permanent ID of message %d in chat %d", tmpID, chatID)
		case typ, ok := <-listener.Updates:
			if !ok {
				return nil, fmt.Errorf("listener closed while waiting for permanent ID")
			}
			u, ok := typ.(*client.UpdateMessageSendSucceeded)
			if !ok || u.OldMessageId != tmpID {
				continue
			}
			r.logger.Debug("Permanent ID received",
				slog.Int64("chat_id", chatID),
				slog.Int64("tmp_id", tmpID),
				slog.Int64("permanent_id", u.Message.Id),
			)
			return mapMessage(u.Message), nil
		}
	}
}

// SendMessageAlbum отправляет медиа-альбом.
func (r *Repo) SendMessageAlbum(_ context.Context, chatID domain.ChatID, contents []domain.InputMessageContent) ([]domain.MessageID, error) {
	tdContents := make([]client.InputMessageContent, 0, len(contents))
	for _, c := range contents {
		tdContents = append(tdContents, toTDLibInputMessageContent(c))
	}

	msgs, err := r.tdClient.SendMessageAlbum(&client.SendMessageAlbumRequest{
		ChatId:               chatID,
		InputMessageContents: tdContents,
	})
	if err != nil {
		return nil, fmt.Errorf("send message album: %w", err)
	}

	ids := make([]domain.MessageID, 0, len(msgs.Messages))
	for _, m := range msgs.Messages {
		if m != nil {
			ids = append(ids, m.Id)
		}
	}
	return ids, nil
}

// ForwardMessages пересылает сообщения из одного чата в другой.
func (r *Repo) ForwardMessages(_ context.Context, fromChatID, toChatID domain.ChatID, messageIDs []domain.MessageID) ([]domain.MessageID, error) {
	msgs, err := r.tdClient.ForwardMessages(&client.ForwardMessagesRequest{
		ChatId:     toChatID,
		FromChatId: fromChatID,
		MessageIds: messageIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("forward messages: %w", err)
	}

	ids := make([]domain.MessageID, 0, len(msgs.Messages))
	for _, m := range msgs.Messages {
		if m != nil {
			ids = append(ids, m.Id)
		}
	}
	return ids, nil
}

// GetMessage возвращает сообщение по ID.
func (r *Repo) GetMessage(_ context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error) {
	msg, err := r.tdClient.GetMessage(&client.GetMessageRequest{
		ChatId:    chatID,
		MessageId: messageID,
	})
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	return mapMessage(msg), nil
}

// EditMessageText редактирует текст сообщения.
func (r *Repo) EditMessageText(_ context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error {
	_, err := r.tdClient.EditMessageText(&client.EditMessageTextRequest{
		ChatId:    chatID,
		MessageId: messageID,
		InputMessageContent: &client.InputMessageText{
			Text: toTDLibFormattedText(text),
		},
	})
	if err != nil {
		return fmt.Errorf("edit message text: %w", err)
	}
	return nil
}

// EditMessageCaption редактирует подпись медиа-сообщения.
func (r *Repo) EditMessageCaption(_ context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error {
	_, err := r.tdClient.EditMessageCaption(&client.EditMessageCaptionRequest{
		ChatId:    chatID,
		MessageId: messageID,
		Caption:   toTDLibFormattedText(text),
	})
	if err != nil {
		return fmt.Errorf("edit message caption: %w", err)
	}
	return nil
}

// DeleteMessages удаляет сообщения из чата.
func (r *Repo) DeleteMessages(_ context.Context, chatID domain.ChatID, messageIDs []domain.MessageID, revoke bool) error {
	_, err := r.tdClient.DeleteMessages(&client.DeleteMessagesRequest{
		ChatId:     chatID,
		MessageIds: messageIDs,
		Revoke:     revoke,
	})
	if err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	return nil
}

// --- Операции со ссылками ---

// GetMessageLink возвращает ссылку на сообщение.
func (r *Repo) GetMessageLink(_ context.Context, chatID domain.ChatID, messageID domain.MessageID, forAlbum bool) (string, error) {
	link, err := r.tdClient.GetMessageLink(&client.GetMessageLinkRequest{
		ChatId:    chatID,
		MessageId: messageID,
		ForAlbum:  forAlbum,
	})
	if err != nil {
		return "", fmt.Errorf("get message link: %w", err)
	}
	return link.Link, nil
}

// GetMessageLinkInfo возвращает информацию о ссылке на сообщение.
func (r *Repo) GetMessageLinkInfo(_ context.Context, url string) (*domain.MessageLinkInfo, error) {
	info, err := r.tdClient.GetMessageLinkInfo(&client.GetMessageLinkInfoRequest{
		Url: url,
	})
	if err != nil {
		return nil, fmt.Errorf("get message link info: %w", err)
	}
	result := &domain.MessageLinkInfo{
		ChatID: info.ChatId,
	}
	if info.Message != nil {
		result.MessageID = info.Message.Id
	}
	return result, nil
}

// --- Операции с текстом ---

// TranslateText переводит текст на указанный язык.
func (r *Repo) TranslateText(_ context.Context, text *domain.FormattedText, lang string) (*domain.FormattedText, error) {
	result, err := r.tdClient.TranslateText(&client.TranslateTextRequest{
		Text:           toTDLibFormattedText(text),
		ToLanguageCode: lang,
	})
	if err != nil {
		return nil, fmt.Errorf("translate text: %w", err)
	}
	return mapFormattedText(result), nil
}

// GetCallbackQueryAnswer получает ответ на callback-запрос.
func (r *Repo) GetCallbackQueryAnswer(_ context.Context, chatID domain.ChatID, messageID domain.MessageID, payload []byte) (string, error) {
	answer, err := r.tdClient.GetCallbackQueryAnswer(&client.GetCallbackQueryAnswerRequest{
		ChatId:    chatID,
		MessageId: messageID,
		Payload:   &client.CallbackQueryPayloadData{Data: payload},
	})
	if err != nil {
		return "", fmt.Errorf("get callback query answer: %w", err)
	}
	return answer.Text, nil
}

// ParseTextEntities парсит текст с разметкой Markdown v2.
func (r *Repo) ParseTextEntities(_ context.Context, text string) (*domain.FormattedText, error) {
	result, err := client.ParseTextEntities(&client.ParseTextEntitiesRequest{
		Text:      text,
		ParseMode: &client.TextParseModeMarkdown{Version: 2},
	})
	if err != nil {
		return nil, fmt.Errorf("parse text entities: %w", err)
	}
	return mapFormattedText(result), nil
}

// GetMarkdownText конвертирует FormattedText в Markdown-представление.
func (r *Repo) GetMarkdownText(_ context.Context, text *domain.FormattedText) (*domain.FormattedText, error) {
	result, err := client.GetMarkdownText(&client.GetMarkdownTextRequest{
		Text: toTDLibFormattedText(text),
	})
	if err != nil {
		return nil, fmt.Errorf("get markdown text: %w", err)
	}
	return mapFormattedText(result), nil
}

// --- Операции с чатами ---

// LoadChats загружает список чатов.
func (r *Repo) LoadChats(_ context.Context, limit int32) error {
	_, err := r.tdClient.LoadChats(&client.LoadChatsRequest{
		Limit: limit,
	})
	if err != nil {
		return fmt.Errorf("load chats: %w", err)
	}
	return nil
}

// WarmUpChat загружает историю чата для прогрева кеша.
func (r *Repo) WarmUpChat(_ context.Context, chatID domain.ChatID, limit int32) error {
	_, err := r.tdClient.GetChatHistory(&client.GetChatHistoryRequest{
		ChatId: chatID,
		Limit:  limit,
	})
	if err != nil {
		return fmt.Errorf("warm up chat: %w", err)
	}
	return nil
}

// GetChatHistory возвращает сообщения чата с пагинацией.
func (r *Repo) GetChatHistory(_ context.Context, chatID domain.ChatID, fromMessageID domain.MessageID, offset int32, limit int32) ([]*domain.Message, error) {
	msgs, err := r.tdClient.GetChatHistory(&client.GetChatHistoryRequest{
		ChatId:        chatID,
		FromMessageId: fromMessageID,
		Offset:        offset,
		Limit:         limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get chat history: %w", err)
	}

	result := make([]*domain.Message, 0, len(msgs.Messages))
	for _, m := range msgs.Messages {
		result = append(result, mapMessage(m))
	}
	return result, nil
}

// GetChatType возвращает тип чата.
func (r *Repo) GetChatType(_ context.Context, chatID domain.ChatID) (string, error) {
	chat, err := r.tdClient.GetChat(&client.GetChatRequest{
		ChatId: chatID,
	})
	if err != nil {
		return "", fmt.Errorf("get chat: %w", err)
	}
	switch chat.Type.(type) {
	case *client.ChatTypeSupergroup:
		return "supergroup", nil
	case *client.ChatTypeBasicGroup:
		return "basicGroup", nil
	default:
		return "private", nil
	}
}

// --- Системные операции ---

// GetOption возвращает значение опции TDLib.
func (r *Repo) GetOption(_ context.Context, name string) (string, error) {
	opt, err := client.GetOption(&client.GetOptionRequest{Name: name})
	if err != nil {
		return "", fmt.Errorf("get option: %w", err)
	}
	if s, ok := opt.(*client.OptionValueString); ok {
		return s.Value, nil
	}
	if i, ok := opt.(*client.OptionValueInteger); ok {
		return fmt.Sprintf("%d", i.Value), nil
	}
	return "", nil
}

// GetMe возвращает информацию о текущем пользователе.
func (r *Repo) GetMe(_ context.Context) (int64, error) {
	user, err := r.tdClient.GetMe()
	if err != nil {
		return 0, fmt.Errorf("get me: %w", err)
	}
	return user.Id, nil
}

// --- Batch-операции ---

// GetMessages возвращает сообщения по списку ID (batch).
func (r *Repo) GetMessages(_ context.Context, chatID domain.ChatID, messageIDs []domain.MessageID) ([]*domain.Message, error) {
	msgs, err := r.tdClient.GetMessages(&client.GetMessagesRequest{
		ChatId:     chatID,
		MessageIds: messageIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	result := make([]*domain.Message, 0, len(msgs.Messages))
	for _, m := range msgs.Messages {
		result = append(result, mapMessage(m))
	}
	return result, nil
}

// --- Управление чатами (stand) ---

// CreateNewSupergroupChat создаёт новый канал или супергруппу.
func (r *Repo) CreateNewSupergroupChat(_ context.Context, title string, isChannel bool, description string) (domain.ChatID, int64, error) {
	chat, err := r.tdClient.CreateNewSupergroupChat(&client.CreateNewSupergroupChatRequest{
		Title:       title,
		IsChannel:   isChannel,
		Description: description,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("create supergroup chat: %w", err)
	}

	var supergroupID int64
	if sg, ok := chat.Type.(*client.ChatTypeSupergroup); ok {
		supergroupID = sg.SupergroupId
	}
	return chat.Id, supergroupID, nil
}

// CreateNewBasicGroupChat создаёт новую базовую группу.
func (r *Repo) CreateNewBasicGroupChat(_ context.Context, title string, userIDs []int64) (domain.ChatID, error) {
	result, err := r.tdClient.CreateNewBasicGroupChat(&client.CreateNewBasicGroupChatRequest{
		Title:   title,
		UserIds: userIDs,
	})
	if err != nil {
		return 0, fmt.Errorf("create basic group chat: %w", err)
	}
	return result.ChatId, nil
}

// SetSupergroupUsername устанавливает username для супергруппы или канала.
func (r *Repo) SetSupergroupUsername(_ context.Context, supergroupID int64, username string) error {
	_, err := r.tdClient.SetSupergroupUsername(&client.SetSupergroupUsernameRequest{
		SupergroupId: supergroupID,
		Username:     username,
	})
	if err != nil {
		return fmt.Errorf("set supergroup username: %w", err)
	}
	return nil
}

// DeleteChat удаляет чат.
func (r *Repo) DeleteChat(_ context.Context, chatID domain.ChatID) error {
	_, err := r.tdClient.DeleteChat(&client.DeleteChatRequest{
		ChatId: chatID,
	})
	if err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}
	return nil
}
