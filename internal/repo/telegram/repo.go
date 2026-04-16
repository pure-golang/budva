package telegram

import (
	"context"
	"log/slog"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
)

// authService — интерфейс для управления состоянием авторизации.
type authService interface {
	SetState(state domain.AuthorizationState, extra any)
	ReadChan() <-chan string
}

// Repo реализует взаимодействие с Telegram через TDLib.
// Текущая версия работает без TDLib; реальная TDLib-интеграция будет добавлена в Phase B.
type Repo struct {
	clientAdapter
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

// Start инициализирует репозиторий (без TDLib).
func (r *Repo) Start(_ context.Context) error {
	r.logger.Info("Telegram repo started")
	return nil
}

// RunAuthFlow запускает state machine авторизации.
// В Phase B этот метод будет заменён на реальный TDLib flow.
func (r *Repo) RunAuthFlow(ctx context.Context, auth authService) {
	// WaitPhone
	auth.SetState(domain.AuthStateWaitPhone, nil)
	phone := r.readInputOrCancel(ctx, auth)
	if phone == "" {
		return
	}
	r.logger.Info("Phone submitted", slog.String("phone", domain.MaskPhoneNumber(phone)))

	// WaitCode
	auth.SetState(domain.AuthStateWaitCode, nil)
	code := r.readInputOrCancel(ctx, auth)
	if code == "" {
		return
	}
	r.logger.Info("Code submitted")

	// WaitPassword (в реальном TDLib этот шаг опционален — зависит от 2FA)
	auth.SetState(domain.AuthStateWaitPassword, &domain.WaitPasswordState{
		PasswordHint: "2FA password",
	})
	password := r.readInputOrCancel(ctx, auth)
	if password == "" {
		return
	}
	r.logger.Info("Password submitted")

	// Ready
	auth.SetState(domain.AuthStateReady, nil)
	close(r.clientDone)
	r.logger.Info("Authorization complete")
}

func (r *Repo) readInputOrCancel(ctx context.Context, auth authService) string {
	select {
	case <-ctx.Done():
		return ""
	case input := <-auth.ReadChan():
		return input
	}
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
