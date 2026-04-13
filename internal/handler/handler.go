package handler

import (
	"log/slog"

	"github.com/pure-golang/budva/internal/domain"
)

// Handler обрабатывает обновления Telegram.
type Handler struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр обработчика обновлений.
func New(logger *slog.Logger) *Handler {
	return &Handler{
		logger: logger.With("module", "handler"),
	}
}

// OnNewMessage обрабатывает новое сообщение.
func (h *Handler) OnNewMessage(chatID domain.ChatID, messageID domain.MessageID, text string) {
	h.logger.Debug("New message", "chat_id", chatID, "message_id", messageID)
}

// OnEditedMessage обрабатывает отредактированное сообщение.
func (h *Handler) OnEditedMessage(chatID domain.ChatID, messageID domain.MessageID, text string) {
	h.logger.Debug("Edited message", "chat_id", chatID, "message_id", messageID)
}

// OnDeletedMessages обрабатывает удаление сообщений.
func (h *Handler) OnDeletedMessages(chatID domain.ChatID, messageIDs []domain.MessageID) {
	h.logger.Debug("Deleted messages", "chat_id", chatID, "count", len(messageIDs))
}

// OnMessageSendSucceeded обрабатывает подтверждение отправки (temp→real ID mapping).
func (h *Handler) OnMessageSendSucceeded(chatID domain.ChatID, tempID domain.MessageID, realID domain.MessageID) {
	h.logger.Debug("Message send succeeded", "chat_id", chatID, "temp_id", tempID, "real_id", realID)
}
