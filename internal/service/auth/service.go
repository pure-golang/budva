package auth

import (
	"log/slog"
	"sync"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// StateListener получает уведомления об изменении состояния авторизации.
type StateListener = func(state domain.AuthorizationState, extra any)

// Service управляет авторизацией в Telegram.
type Service struct {
	logger    *slog.Logger
	mu        sync.RWMutex
	state     domain.AuthorizationState
	listeners []StateListener
	inputChan chan string
}

// New создаёт новый экземпляр сервиса авторизации.
func New() *Service {
	return &Service{
		logger:    slog.Default().With("module", "service.auth"),
		inputChan: make(chan string, 1),
	}
}

// Subscribe добавляет подписчика на изменение состояния.
func (s *Service) Subscribe(listener StateListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, listener)
}

// SetState устанавливает новое состояние и оповещает подписчиков.
func (s *Service) SetState(state domain.AuthorizationState, extra any) {
	s.mu.Lock()
	s.state = state
	listeners := make([]StateListener, len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.Unlock()

	s.logger.Info("Auth state changed", slog.String("state", state.String()))
	for _, l := range listeners {
		l(state, extra)
	}
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

// ReadInput ожидает ввод от пользователя (блокирует).
func (s *Service) ReadInput() string {
	return <-s.inputChan
}
