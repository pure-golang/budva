package message

import (
	"log/slog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// Service извлекает и формирует контент сообщений.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр сервиса сообщений.
func New() *Service {
	return &Service{
		logger: slog.Default().With("module", "service.message"),
	}
}

// GetFormattedText извлекает FormattedText из содержимого сообщения.
// Возвращает nil для неподдерживаемых типов контента.
func (s *Service) GetFormattedText(msg *domain.Message) *domain.FormattedText {
	if msg == nil || msg.Content.Type == domain.ContentSystem || msg.Content.Type == domain.ContentUnknown {
		return nil
	}
	return msg.Content.Text
}

// IsSystemMessage проверяет, является ли сообщение системным.
func (s *Service) IsSystemMessage(msg *domain.Message) bool {
	if msg == nil {
		return false
	}
	return msg.Content.Type == domain.ContentSystem
}

// GetReplyMarkupData извлекает callback-данные из inline-клавиатуры.
func (s *Service) GetReplyMarkupData(msg *domain.Message) []byte {
	if msg == nil || msg.ReplyMarkup == nil {
		return nil
	}
	return msg.ReplyMarkup.CallbackData
}

// BuildInputContent формирует InputMessageContent из сообщения-оригинала.
func (s *Service) BuildInputContent(msg *domain.Message, text *domain.FormattedText) domain.InputMessageContent {
	c := msg.Content
	input := domain.InputMessageContent{
		Type:               c.Type,
		Text:               text,
		FileID:             c.FileID,
		ThumbnailFileID:    c.ThumbnailFileID,
		Width:              c.Width,
		Height:             c.Height,
		Duration:           c.Duration,
		FileName:           c.FileName,
		MimeType:           c.MimeType,
		DisableLinkPreview: c.DisableLinkPreview,
	}
	return input
}
