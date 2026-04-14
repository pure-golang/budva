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
// Для текстовых сообщений инвертирует DisableLinkPreview (если был preview, отключаем его в копии).
// Для медиа-сообщений копирует FileID, размеры, длительность и thumbnail.
func (s *Service) BuildInputContent(msg *domain.Message, text *domain.FormattedText) domain.InputMessageContent {
	c := msg.Content
	input := domain.InputMessageContent{
		Type: c.Type,
		Text: text,
	}

	switch c.Type {
	case domain.ContentText:
		input.DisableLinkPreview = !c.DisableLinkPreview
	case domain.ContentPhoto:
		input.FileID = c.FileID
		input.ThumbnailFileID = c.ThumbnailFileID
		input.Width = c.Width
		input.Height = c.Height
	case domain.ContentVideo:
		input.FileID = c.FileID
		input.ThumbnailFileID = c.ThumbnailFileID
		input.Width = c.Width
		input.Height = c.Height
		input.Duration = c.Duration
	case domain.ContentDocument:
		input.FileID = c.FileID
		input.ThumbnailFileID = c.ThumbnailFileID
		input.FileName = c.FileName
		input.MimeType = c.MimeType
	case domain.ContentAudio:
		input.FileID = c.FileID
		input.ThumbnailFileID = c.ThumbnailFileID
		input.Duration = c.Duration
		input.FileName = c.FileName
		input.MimeType = c.MimeType
	case domain.ContentAnimation:
		input.FileID = c.FileID
		input.ThumbnailFileID = c.ThumbnailFileID
		input.Width = c.Width
		input.Height = c.Height
		input.Duration = c.Duration
	case domain.ContentVoiceNote:
		input.Duration = c.Duration
	}

	return input
}
