package dedup

import (
	"sync"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// Tracker отслеживает, в какие целевые чаты уже было отправлено сообщение.
type Tracker struct {
	mu        sync.Mutex
	forwarded map[domain.ChatID]bool
}

// NewTracker создаёт трекер для указанного набора целевых чатов.
func NewTracker(destinations []domain.ChatID) *Tracker {
	m := make(map[domain.ChatID]bool, len(destinations))
	for _, id := range destinations {
		m[id] = false
	}
	return &Tracker{forwarded: m}
}

// TryMark помечает чат как получивший сообщение.
// Возвращает true, если чат ещё не был помечен.
func (t *Tracker) TryMark(chatID domain.ChatID) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.forwarded[chatID] {
		return false
	}
	t.forwarded[chatID] = true
	return true
}

// NewTrackerFactory возвращает фабрику для создания Tracker.
func NewTrackerFactory() func(destinations []domain.ChatID) *Tracker {
	return NewTracker
}
