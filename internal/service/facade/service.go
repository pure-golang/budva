package facade

import (
	"context"
	"log/slog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

type telegramGateway interface {
	GetMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error)
	SendMessage(ctx context.Context, chatID domain.ChatID, content domain.InputMessageContent) (domain.MessageID, error)
	SendMessageAlbum(ctx context.Context, chatID domain.ChatID, contents []domain.InputMessageContent) ([]domain.MessageID, error)
	ForwardMessages(ctx context.Context, fromChatID domain.ChatID, toChatID domain.ChatID, messageIDs []domain.MessageID) ([]domain.MessageID, error)
	EditMessageText(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text *domain.FormattedText) error
	DeleteMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID, revoke bool) error
	GetMessageLink(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (string, error)
	GetMessageLinkInfo(ctx context.Context, url string) (*domain.MessageLinkInfo, error)
	GetOption(ctx context.Context, name string) (string, error)
	GetMe(ctx context.Context) (int64, error)
}

// Service реализует фасад для внешнего доступа к Telegram.
type Service struct {
	logger   *slog.Logger
	telegram telegramGateway
}

// New создаёт новый экземпляр фасада.
func New(telegram telegramGateway) *Service {
	return &Service{
		logger:   slog.Default().With("module", "service.facade"),
		telegram: telegram,
	}
}

// GetMessage возвращает сообщение по ID.
func (s *Service) GetMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error) {
	return s.telegram.GetMessage(ctx, chatID, messageID)
}

// SendMessage отправляет текстовое сообщение.
func (s *Service) SendMessage(ctx context.Context, chatID domain.ChatID, text string) error {
	content := domain.InputMessageContent{
		Type: domain.ContentText,
		Text: &domain.FormattedText{Text: text},
	}
	_, err := s.telegram.SendMessage(ctx, chatID, content)
	return err
}

// SendMessageAlbum отправляет несколько сообщений как альбом.
func (s *Service) SendMessageAlbum(ctx context.Context, chatID domain.ChatID, texts []string) error {
	var contents []domain.InputMessageContent
	for _, text := range texts {
		contents = append(contents, domain.InputMessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: text},
		})
	}
	_, err := s.telegram.SendMessageAlbum(ctx, chatID, contents)
	return err
}

// ForwardMessage пересылает сообщение из одного чата в другой.
func (s *Service) ForwardMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) error {
	// Пересылаем в тот же чат (копия) — конкретный destination определяет клиент
	_, err := s.telegram.ForwardMessages(ctx, chatID, chatID, []domain.MessageID{messageID})
	return err
}

// UpdateMessage обновляет текст сообщения.
func (s *Service) UpdateMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text string) error {
	return s.telegram.EditMessageText(ctx, chatID, messageID, &domain.FormattedText{Text: text})
}

// DeleteMessages удаляет сообщения.
func (s *Service) DeleteMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID) error {
	return s.telegram.DeleteMessages(ctx, chatID, messageIDs, true)
}

// GetMessageLink возвращает публичную ссылку на сообщение.
func (s *Service) GetMessageLink(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (string, error) {
	return s.telegram.GetMessageLink(ctx, chatID, messageID)
}

// GetMessageLinkInfo извлекает информацию о сообщении по ссылке.
func (s *Service) GetMessageLinkInfo(ctx context.Context, link string) (*domain.MessageLinkInfo, error) {
	return s.telegram.GetMessageLinkInfo(ctx, link)
}

// Status содержит информацию о состоянии сервиса.
type Status struct {
	TDLibVersion string
	UserID       int64
}

// GetStatus возвращает текущий статус сервиса.
func (s *Service) GetStatus(ctx context.Context) (*Status, error) {
	version, err := s.telegram.GetOption(ctx, "version")
	if err != nil {
		return nil, err
	}
	userID, err := s.telegram.GetMe(ctx)
	if err != nil {
		return nil, err
	}
	return &Status{
		TDLibVersion: version,
		UserID:       userID,
	}, nil
}
