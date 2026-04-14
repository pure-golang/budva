// Package support содержит shared test infrastructure для integration/bdd/e2e слоёв.
package support

import (
	"context"
	"fmt"
	"sync"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// FakeTelegram — stateful in-memory реализация telegramGateway для тестов.
// Хранит сообщения, позволяет проверять доставку между steps.
type FakeTelegram struct {
	mu         sync.Mutex
	messages   map[domain.ChatID]map[domain.MessageID]*domain.Message
	nextMsgID  int64
	clientDone chan struct{}
}

// NewFakeTelegram создаёт новый экземпляр.
func NewFakeTelegram() *FakeTelegram {
	done := make(chan struct{})
	close(done)
	return &FakeTelegram{
		messages:   make(map[domain.ChatID]map[domain.MessageID]*domain.Message),
		nextMsgID:  1000,
		clientDone: done,
	}
}

// ClientDone возвращает закрытый канал (клиент всегда готов).
func (f *FakeTelegram) ClientDone() <-chan struct{} {
	return f.clientDone
}

// GetOption возвращает фиктивные опции.
func (f *FakeTelegram) GetOption(_ context.Context, name string) (string, error) {
	if name == "version" {
		return "test-1.0.0", nil
	}
	return "", nil
}

// GetMe возвращает фиктивный user ID.
func (f *FakeTelegram) GetMe(_ context.Context) (int64, error) {
	return 999999, nil
}

// SendMessage сохраняет сообщение и возвращает temp ID.
func (f *FakeTelegram) SendMessage(_ context.Context, chatID domain.ChatID, content domain.InputMessageContent) (domain.MessageID, error) { //nolint:gocritic // InputMessageContent by value for interface compatibility
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextMsgID++
	msgID := f.nextMsgID

	msg := &domain.Message{
		ChatID: chatID,
		ID:     msgID,
		Content: domain.MessageContent{
			Type: content.Type,
			Text: content.Text,
		},
	}
	// Сохраняем ReplyTo для проверки reply chain в тестах
	if content.ReplyToMessageID != 0 {
		msg.ReplyTo = &domain.MessageReplyTo{
			ChatID:    chatID,
			MessageID: content.ReplyToMessageID,
		}
	}

	if f.messages[chatID] == nil {
		f.messages[chatID] = make(map[domain.MessageID]*domain.Message)
	}
	f.messages[chatID][msgID] = msg

	return msgID, nil
}

// SendMessageAlbum сохраняет несколько сообщений.
func (f *FakeTelegram) SendMessageAlbum(_ context.Context, chatID domain.ChatID, contents []domain.InputMessageContent) ([]domain.MessageID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	ids := make([]domain.MessageID, 0, len(contents))
	for _, content := range contents {
		f.nextMsgID++
		msgID := f.nextMsgID
		msg := &domain.Message{
			ChatID:  chatID,
			ID:      msgID,
			Content: domain.MessageContent{Type: content.Type, Text: content.Text},
		}
		if f.messages[chatID] == nil {
			f.messages[chatID] = make(map[domain.MessageID]*domain.Message)
		}
		f.messages[chatID][msgID] = msg
		ids = append(ids, msgID)
	}
	return ids, nil
}

// ForwardMessages копирует сообщения из одного чата в другой.
func (f *FakeTelegram) ForwardMessages(_ context.Context, fromChatID domain.ChatID, toChatID domain.ChatID, messageIDs []domain.MessageID) ([]domain.MessageID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var newIDs []domain.MessageID
	for _, srcID := range messageIDs {
		srcMsg := f.getMessage(fromChatID, srcID)
		if srcMsg == nil {
			continue
		}
		f.nextMsgID++
		newID := f.nextMsgID
		newMsg := &domain.Message{
			ChatID:      toChatID,
			ID:          newID,
			Content:     srcMsg.Content,
			ForwardInfo: &domain.MessageForwardInfo{OriginChatID: fromChatID, OriginMessageID: srcID},
		}
		if f.messages[toChatID] == nil {
			f.messages[toChatID] = make(map[domain.MessageID]*domain.Message)
		}
		f.messages[toChatID][newID] = newMsg
		newIDs = append(newIDs, newID)
	}
	return newIDs, nil
}

// GetMessage возвращает сообщение из store.
func (f *FakeTelegram) GetMessage(_ context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	msg := f.getMessage(chatID, messageID)
	if msg == nil {
		return nil, fmt.Errorf("message %d not found in chat %d", messageID, chatID)
	}
	return msg, nil
}

// EditMessageText обновляет текст сообщения.
func (f *FakeTelegram) EditMessageText(_ context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	msg := f.getMessage(chatID, messageID)
	if msg == nil {
		return fmt.Errorf("message %d not found in chat %d", messageID, chatID)
	}
	msg.Content.Text = text
	return nil
}

// EditMessageCaption обновляет подпись медиа-сообщения.
func (f *FakeTelegram) EditMessageCaption(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error {
	return f.EditMessageText(ctx, chatID, messageID, text)
}

// DeleteMessages удаляет сообщения из store.
func (f *FakeTelegram) DeleteMessages(_ context.Context, chatID domain.ChatID, messageIDs []domain.MessageID, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, id := range messageIDs {
		if f.messages[chatID] != nil {
			delete(f.messages[chatID], id)
		}
	}
	return nil
}

// GetMessageLink возвращает фиктивную ссылку.
func (f *FakeTelegram) GetMessageLink(_ context.Context, chatID domain.ChatID, messageID domain.MessageID) (string, error) {
	return fmt.Sprintf("https://t.me/c/%d/%d", chatID, messageID), nil
}

// GetMessageLinkInfo парсит фиктивную ссылку.
func (f *FakeTelegram) GetMessageLinkInfo(_ context.Context, _ string) (*domain.MessageLinkInfo, error) {
	return nil, fmt.Errorf("link info not supported in fake")
}

// TranslateText возвращает переведённый текст (добавляет prefix).
func (f *FakeTelegram) TranslateText(_ context.Context, text *domain.FormattedText, lang string) (*domain.FormattedText, error) {
	return &domain.FormattedText{
		Text:     fmt.Sprintf("[%s] %s", lang, text.Text),
		Entities: text.Entities,
	}, nil
}

// GetCallbackQueryAnswer возвращает пустой ответ.
func (f *FakeTelegram) GetCallbackQueryAnswer(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ []byte) (string, error) {
	return "", nil
}

// GetChatType всегда возвращает "supergroup".
func (f *FakeTelegram) GetChatType(_ context.Context, _ domain.ChatID) (string, error) {
	return "supergroup", nil
}

// ParseTextEntities возвращает текст как есть.
func (f *FakeTelegram) ParseTextEntities(_ context.Context, text string) (*domain.FormattedText, error) {
	return &domain.FormattedText{Text: text}, nil
}

// GetChatHistory возвращает пустой список.
func (f *FakeTelegram) GetChatHistory(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ int32, _ int32) ([]*domain.Message, error) {
	return nil, nil
}

// ReplaceMessageID имитирует замену temporary ID на permanent ID (как в TDLib OnMessageSendSucceeded).
func (f *FakeTelegram) ReplaceMessageID(chatID domain.ChatID, oldID, newID domain.MessageID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.messages[chatID] == nil {
		return
	}
	msg := f.messages[chatID][oldID]
	if msg == nil {
		return
	}
	delete(f.messages[chatID], oldID)
	msg.ID = newID
	f.messages[chatID][newID] = msg
}

// Reset очищает все сообщения (аналог truncateTables между subtests).
func (f *FakeTelegram) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = make(map[domain.ChatID]map[domain.MessageID]*domain.Message)
	f.nextMsgID = 1000
}

// --- Методы для assertions в тестах ---

// PutMessage помещает сообщение в store (для Given-шагов).
func (f *FakeTelegram) PutMessage(msg *domain.Message) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.messages[msg.ChatID] == nil {
		f.messages[msg.ChatID] = make(map[domain.MessageID]*domain.Message)
	}
	f.messages[msg.ChatID][msg.ID] = msg
}

// MessagesInChat возвращает все сообщения в чате (для Then-шагов).
func (f *FakeTelegram) MessagesInChat(chatID domain.ChatID) []*domain.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]*domain.Message, 0, len(f.messages[chatID]))
	for _, msg := range f.messages[chatID] {
		result = append(result, msg)
	}
	return result
}

// HasMessageWithText проверяет наличие сообщения с указанным текстом в чате.
func (f *FakeTelegram) HasMessageWithText(chatID domain.ChatID, text string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, msg := range f.messages[chatID] {
		if msg.Content.Text != nil && msg.Content.Text.Text == text {
			return true
		}
	}
	return false
}

func (f *FakeTelegram) getMessage(chatID domain.ChatID, messageID domain.MessageID) *domain.Message {
	if f.messages[chatID] == nil {
		return nil
	}
	return f.messages[chatID][messageID]
}
