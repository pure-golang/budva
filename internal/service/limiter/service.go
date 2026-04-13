package limiter

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/pure-golang/budva/internal/domain"
)

const interval = 3 * time.Second

// Service ограничивает частоту пересылки в один целевой чат.
type Service struct {
	logger        *slog.Logger
	mu            sync.Mutex
	lastForwarded map[domain.ChatID]time.Time
}

// New создаёт новый экземпляр лимитера.
func New(logger *slog.Logger) *Service {
	return &Service{
		logger:        logger.With("module", "service.limiter"),
		lastForwarded: make(map[domain.ChatID]time.Time),
	}
}

// Wait блокирует до истечения минимального интервала с предыдущей пересылки в чат.
func (s *Service) Wait(ctx context.Context, chatID domain.ChatID) {
	s.mu.Lock()
	diff := time.Since(s.lastForwarded[chatID])
	if diff < interval {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval - diff):
		}
		s.mu.Lock()
	}
	s.lastForwarded[chatID] = time.Now()
	s.mu.Unlock()
}
