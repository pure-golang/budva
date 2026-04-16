package telegram

import (
	"context"
	"log/slog"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
)

// Repo реализует взаимодействие с Telegram через TDLib.
// Текущая версия работает без TDLib; реальная TDLib-интеграция будет добавлена в Phase B.
type Repo struct {
	logger     *slog.Logger
	cfg        config.TelegramConfig
	clientDone chan struct{}
	updates    chan domain.Update
	authStates chan domain.AuthStateEvent
}

// New создаёт новый экземпляр Telegram-репозитория.
func New(cfg config.TelegramConfig) *Repo {
	return &Repo{
		logger:     slog.Default().With("module", "repo.telegram"),
		cfg:        cfg,
		clientDone: make(chan struct{}),
		updates:    make(chan domain.Update, 100),
		authStates: make(chan domain.AuthStateEvent, 10),
	}
}

// Start инициализирует репозиторий и отправляет начальное состояние авторизации.
func (r *Repo) Start(_ context.Context) error {
	r.logger.Info("Telegram repo started")
	r.authStates <- domain.AuthStateEvent{State: domain.AuthStateWaitPhone}
	return nil
}

// Close завершает TDLib-сессию.
func (r *Repo) Close() error {
	return nil
}

// AuthStates возвращает канал событий авторизации.
func (r *Repo) AuthStates() <-chan domain.AuthStateEvent {
	return r.authStates
}

// ClientDone возвращает канал, закрывающийся после авторизации.
func (r *Repo) ClientDone() <-chan struct{} {
	return r.clientDone
}

// Updates возвращает канал обновлений.
func (r *Repo) Updates() <-chan domain.Update {
	return r.updates
}
