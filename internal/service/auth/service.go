package auth

import (
	"context"
	"log/slog"
	"sync"

	"github.com/pure-golang/budva-claude/internal/domain"
)

type telegramRepo interface {
	AuthStates() <-chan domain.AuthStateEvent
	SubmitPhone(ctx context.Context, phone string) error
	SubmitCode(ctx context.Context, code string) error
	SubmitPassword(ctx context.Context, password string) error
}

// StateListener получает уведомления об изменении состояния авторизации.
type StateListener = func(state domain.AuthorizationState, extra any)

// Service управляет авторизацией в Telegram.
type Service struct {
	logger       *slog.Logger
	telegramRepo telegramRepo
	mu           sync.RWMutex
	state        domain.AuthorizationState
	extra        any
	listeners    []StateListener
	inputChan    chan string
}

// New создаёт новый экземпляр сервиса авторизации.
func New(telegramRepo telegramRepo) *Service {
	return &Service{
		logger:       slog.Default().With("module", "service.auth"),
		telegramRepo: telegramRepo,
		inputChan:    make(chan string, 1),
	}
}

// Start запускает оркестрацию авторизационного flow.
func (s *Service) Start(ctx context.Context) {
	go s.run(ctx)
}

func (s *Service) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.telegramRepo.AuthStates():
			// Пропускаем transitional states — не broadcast, не ждём input
			if event.State == domain.AuthStateClosing || event.State == domain.AuthStateClosed {
				continue
			}

			s.setState(event.State, event.Extra)

			if event.State == domain.AuthStateReady {
				return
			}

			// Ожидаем пользовательский ввод и отправляем в repo
			select {
			case <-ctx.Done():
				return
			case input := <-s.inputChan:
				var err error
				switch event.State {
				case domain.AuthStateWaitPhone:
					err = s.telegramRepo.SubmitPhone(ctx, input)
				case domain.AuthStateWaitCode:
					err = s.telegramRepo.SubmitCode(ctx, input)
				case domain.AuthStateWaitPassword:
					err = s.telegramRepo.SubmitPassword(ctx, input)
				}
				if err != nil {
					s.logger.Error("Failed to submit auth input", slog.Any("err", err))
				}
			}
		}
	}
}

func (s *Service) setState(state domain.AuthorizationState, extra any) {
	s.mu.Lock()
	s.state = state
	s.extra = extra
	listeners := make([]StateListener, len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.Unlock()

	s.logger.Info("Auth state changed", slog.String("state", state.String()))
	for _, l := range listeners {
		go l(state, extra)
	}
}

// Subscribe добавляет подписчика на изменение состояния.
func (s *Service) Subscribe(listener StateListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, listener)
}

// Extra возвращает дополнительные данные текущего состояния.
func (s *Service) Extra() any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.extra
}

// State возвращает текущее состояние авторизации.
func (s *Service) State() domain.AuthorizationState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// InputChan возвращает канал для ввода данных авторизации (телефон, код, пароль).
func (s *Service) InputChan() chan<- string {
	return s.inputChan
}

// Close останавливает сервис и закрывает канал ввода.
func (s *Service) Close() error {
	close(s.inputChan)
	return nil
}
