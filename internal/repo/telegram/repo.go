package telegram

import (
	"context"
	"log/slog"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
)

// Repo реализует взаимодействие с Telegram через TDLib.
// Текущая версия — заглушка; реальная реализация будет добавлена позже.
type Repo struct {
	logger     *slog.Logger
	cfg        config.TelegramConfig
	clientDone chan struct{}
	updates    chan domain.Update
}

// New создаёт новый экземпляр Telegram-репозитория.
func New(cfg config.TelegramConfig) *Repo {
	return &Repo{
		logger:     slog.Default().With("module", "repo.telegram"),
		cfg:        cfg,
		clientDone: make(chan struct{}),
		updates:    make(chan domain.Update, 100),
	}
}

// Start инициализирует TDLib-клиент.
func (r *Repo) Start(_ context.Context) error {
	r.logger.Info("Telegram repo started (stub)")
	close(r.clientDone)
	return nil
}

// Close завершает TDLib-сессию.
func (r *Repo) Close() error {
	return nil
}

// ClientDone возвращает канал, закрывающийся после авторизации.
func (r *Repo) ClientDone() <-chan struct{} {
	return r.clientDone
}

// Updates возвращает канал обновлений.
func (r *Repo) Updates() <-chan domain.Update {
	return r.updates
}

// SendMessage отправляет сообщение в чат.
func (r *Repo) SendMessage(_ context.Context, _ domain.ChatID, _ domain.InputMessageContent) (domain.MessageID, error) {
	return 0, nil
}

// SendMessageAlbum отправляет медиа-альбом.
func (r *Repo) SendMessageAlbum(_ context.Context, _ domain.ChatID, _ []domain.InputMessageContent) ([]domain.MessageID, error) {
	return nil, nil
}

// ForwardMessages пересылает сообщения из одного чата в другой.
func (r *Repo) ForwardMessages(_ context.Context, _, _ domain.ChatID, _ []domain.MessageID) ([]domain.MessageID, error) {
	return nil, nil
}

// GetMessage возвращает сообщение по ID.
func (r *Repo) GetMessage(_ context.Context, _ domain.ChatID, _ domain.MessageID) (*domain.Message, error) {
	return nil, nil
}

// EditMessageText редактирует текст сообщения.
func (r *Repo) EditMessageText(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ *domain.FormattedText) error {
	return nil
}

// EditMessageCaption редактирует подпись медиа-сообщения.
func (r *Repo) EditMessageCaption(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ *domain.FormattedText) error {
	return nil
}

// DeleteMessages удаляет сообщения из чата.
func (r *Repo) DeleteMessages(_ context.Context, _ domain.ChatID, _ []domain.MessageID, _ bool) error {
	return nil
}

// GetMessageLink возвращает ссылку на сообщение.
func (r *Repo) GetMessageLink(_ context.Context, _ domain.ChatID, _ domain.MessageID) (string, error) {
	return "", nil
}

// GetMessageLinkInfo возвращает информацию о ссылке на сообщение.
func (r *Repo) GetMessageLinkInfo(_ context.Context, _ string) (*domain.MessageLinkInfo, error) {
	return nil, nil
}

// TranslateText переводит текст на указанный язык.
func (r *Repo) TranslateText(_ context.Context, _ *domain.FormattedText, _ string) (*domain.FormattedText, error) {
	return nil, nil
}

// GetCallbackQueryAnswer получает ответ на callback-запрос.
func (r *Repo) GetCallbackQueryAnswer(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ []byte) (string, error) {
	return "", nil
}

// ParseTextEntities парсит текст с разметкой Markdown v2.
func (r *Repo) ParseTextEntities(_ context.Context, _ string) (*domain.FormattedText, error) {
	return &domain.FormattedText{Text: ""}, nil
}

// LoadChats загружает список чатов.
func (r *Repo) LoadChats(_ context.Context, _ int32) error {
	return nil
}

// WarmUpChat загружает историю чата для прогрева кеша.
func (r *Repo) WarmUpChat(_ context.Context, _ domain.ChatID, _ int32) error {
	return nil
}

// GetChatHistory возвращает сообщения чата с пагинацией.
func (r *Repo) GetChatHistory(_ context.Context, _ domain.ChatID, _ domain.MessageID, _ int32, _ int32) ([]*domain.Message, error) {
	return nil, nil
}

// GetChatType возвращает тип чата (для определения возможности получить ссылку).
func (r *Repo) GetChatType(_ context.Context, _ domain.ChatID) (string, error) {
	return "supergroup", nil
}

// GetOption возвращает значение опции TDLib.
func (r *Repo) GetOption(_ context.Context, _ string) (string, error) {
	return "", nil
}

// GetMe возвращает информацию о текущем пользователе.
func (r *Repo) GetMe(_ context.Context) (int64, error) {
	return 0, nil
}

// SubmitPhone отправляет номер телефона для авторизации.
func (r *Repo) SubmitPhone(_ context.Context, _ string) error {
	return nil
}

// SubmitCode отправляет код подтверждения.
func (r *Repo) SubmitCode(_ context.Context, _ string) error {
	return nil
}

// SubmitPassword отправляет пароль двухфакторной аутентификации.
func (r *Repo) SubmitPassword(_ context.Context, _ string) error {
	return nil
}
