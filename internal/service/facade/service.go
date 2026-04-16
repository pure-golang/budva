package facade

import (
	"context"
	"path/filepath"
	"runtime/debug"

	"github.com/pure-golang/budva-claude/internal/domain"
	dtogql "github.com/pure-golang/budva-claude/internal/dto/graphql"
)

type telegramRepo interface {
	GetMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error)
	GetChatHistory(ctx context.Context, chatID domain.ChatID, fromMessageID domain.MessageID, offset int32, limit int32) ([]*domain.Message, error)
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
	telegramRepo telegramRepo
}

// New создаёт новый экземпляр фасада.
func New(telegramRepo telegramRepo) *Service {
	return &Service{
		telegramRepo: telegramRepo,
	}
}

// GetMessage возвращает сообщение по ID.
func (s *Service) GetMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (*domain.Message, error) {
	return s.telegramRepo.GetMessage(ctx, chatID, messageID)
}

// SendMessage отправляет текстовое сообщение.
func (s *Service) SendMessage(ctx context.Context, chatID domain.ChatID, text string) error {
	content := domain.InputMessageContent{
		Type: domain.ContentText,
		Text: &domain.FormattedText{Text: text},
	}
	_, err := s.telegramRepo.SendMessage(ctx, chatID, content)
	return err
}

// SendMessageAlbum отправляет несколько сообщений как альбом.
func (s *Service) SendMessageAlbum(ctx context.Context, chatID domain.ChatID, items []domain.AlbumItem) error {
	contents := make([]domain.InputMessageContent, 0, len(items))
	for _, item := range items {
		content := domain.InputMessageContent{
			Text: &domain.FormattedText{Text: item.Text},
		}
		if item.FilePath != "" {
			content.Type = domain.ContentTypeByFileExt(filepath.Ext(item.FilePath))
			content.FilePath = item.FilePath
		} else {
			content.Type = domain.ContentText
		}
		contents = append(contents, content)
	}
	_, err := s.telegramRepo.SendMessageAlbum(ctx, chatID, contents)
	return err
}

// ForwardMessage пересылает сообщение из одного чата в другой.
func (s *Service) ForwardMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) error {
	// Пересылаем в тот же чат (копия) — конкретный destination определяет клиент
	_, err := s.telegramRepo.ForwardMessages(ctx, chatID, chatID, []domain.MessageID{messageID})
	return err
}

// UpdateMessage обновляет текст сообщения.
func (s *Service) UpdateMessage(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, text string) error {
	return s.telegramRepo.EditMessageText(ctx, chatID, messageID, &domain.FormattedText{Text: text})
}

// DeleteMessages удаляет сообщения.
func (s *Service) DeleteMessages(ctx context.Context, chatID domain.ChatID, messageIDs []domain.MessageID) error {
	return s.telegramRepo.DeleteMessages(ctx, chatID, messageIDs, true)
}

// GetChatHistory возвращает сообщения чата с пагинацией.
func (s *Service) GetChatHistory(ctx context.Context, chatID domain.ChatID, fromMessageID domain.MessageID, offset, limit int32) ([]*domain.Message, error) {
	return s.telegramRepo.GetChatHistory(ctx, chatID, fromMessageID, offset, limit)
}

// GetMessageLink возвращает публичную ссылку на сообщение.
func (s *Service) GetMessageLink(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (string, error) {
	return s.telegramRepo.GetMessageLink(ctx, chatID, messageID)
}

// GetMessageLinkInfo извлекает информацию о сообщении по ссылке.
func (s *Service) GetMessageLinkInfo(ctx context.Context, link string) (*domain.MessageLinkInfo, error) {
	return s.telegramRepo.GetMessageLinkInfo(ctx, link)
}

func releaseVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "unknown"
	}
	return info.Main.Version
}

// GetStatus возвращает текущий статус сервиса.
func (s *Service) GetStatus(ctx context.Context) (*dtogql.StatusResponse, error) {
	version, err := s.telegramRepo.GetOption(ctx, "version")
	if err != nil {
		return nil, err
	}
	userID, err := s.telegramRepo.GetMe(ctx)
	if err != nil {
		return nil, err
	}
	return &dtogql.StatusResponse{
		ReleaseVersion: releaseVersion(),
		TDLibVersion:   version,
		UserID:         userID,
	}, nil
}
