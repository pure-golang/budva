package telegram

import (
	"context"
	"log/slog"

	"github.com/pure-golang/budva/internal/config"
	"github.com/pure-golang/budva/internal/domain"
)

// Repo реализует взаимодействие с Telegram через TDLib.
// Текущая версия — заглушка, реальная реализация будет добавлена позже.
type Repo struct {
	logger *slog.Logger
	cfg    config.TelegramConfig
}

// New создаёт новый экземпляр Telegram-репозитория.
func New(cfg config.TelegramConfig, logger *slog.Logger) *Repo {
	return &Repo{
		logger: logger.With("module", "repo.telegram"),
		cfg:    cfg,
	}
}

// Start инициализирует TDLib-клиент.
func (r *Repo) Start(_ context.Context) error {
	r.logger.Info("Telegram repo started (stub)")
	return nil
}

// Close завершает TDLib-сессию.
func (r *Repo) Close() error {
	return nil
}

// SendMessage отправляет сообщение в чат.
func (r *Repo) SendMessage(_ context.Context, _ domain.ChatID, _ string) (domain.MessageID, error) {
	return 0, nil
}

// ForwardMessage пересылает сообщение из одного чата в другой.
func (r *Repo) ForwardMessage(_ context.Context, _, _ domain.ChatID, _ domain.MessageID) (domain.MessageID, error) {
	return 0, nil
}

// EditMessage редактирует сообщение в чате.
func (r *Repo) EditMessage(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ string) error {
	return nil
}

// DeleteMessages удаляет сообщения из чата.
func (r *Repo) DeleteMessages(_ context.Context, _ domain.ChatID, _ []domain.MessageID) error {
	return nil
}

// GetMessageLink возвращает ссылку на сообщение.
func (r *Repo) GetMessageLink(_ context.Context, _ domain.ChatID, _ domain.MessageID) (string, error) {
	return "", nil
}

// TranslateText переводит текст на указанный язык.
func (r *Repo) TranslateText(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

// LoadChats загружает список чатов.
func (r *Repo) LoadChats(_ context.Context, _ int32) error {
	return nil
}

// GetChatHistory загружает историю чата для прогрева.
func (r *Repo) GetChatHistory(_ context.Context, _ domain.ChatID, _ int32) error {
	return nil
}
