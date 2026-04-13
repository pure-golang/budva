package auth

import (
	"log/slog"
	"sync"
)

// AuthState описывает состояние авторизации.
type AuthState string

const (
	StateWaitPhone    AuthState = "waitPhone"
	StateWaitCode     AuthState = "waitCode"
	StateWaitPassword AuthState = "waitPassword"
	StateReady        AuthState = "ready"
	StateClosed       AuthState = "closed"
)

// StateListener получает уведомления об изменении состояния авторизации.
type StateListener = func(state AuthState)

// Service управляет авторизацией в Telegram.
type Service struct {
	logger    *slog.Logger
	mu        sync.RWMutex
	state     AuthState
	listeners []StateListener
}

// New создаёт новый экземпляр сервиса авторизации.
func New(logger *slog.Logger) *Service {
	return &Service{
		logger: logger.With("module", "service.auth"),
	}
}

// Subscribe добавляет подписчика на изменение состояния.
func (s *Service) Subscribe(listener StateListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, listener)
}

// SetState устанавливает новое состояние и оповещает подписчиков.
func (s *Service) SetState(state AuthState) {
	s.mu.Lock()
	s.state = state
	listeners := make([]StateListener, len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.Unlock()

	s.logger.Info("Auth state changed", "state", string(state))
	for _, l := range listeners {
		l(state)
	}
}

// State возвращает текущее состояние авторизации.
func (s *Service) State() AuthState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}
