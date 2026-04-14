package album

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pure-golang/budva-claude/internal/domain"
)

type entry struct {
	messageIDs   []domain.MessageID
	lastReceived time.Time
}

// Service группирует сообщения медиа-альбомов.
type Service struct {
	logger *slog.Logger
	mu     sync.Mutex
	albums map[domain.MediaAlbumKey]*entry
}

// New создаёт новый экземпляр сервиса медиа-альбомов.
func New() *Service {
	return &Service{
		logger: slog.Default().With("module", "service.album"),
		albums: make(map[domain.MediaAlbumKey]*entry),
	}
}

// AddMessage добавляет сообщение в альбом.
// Возвращает true, если это первое сообщение в альбоме.
func (s *Service) AddMessage(key domain.MediaAlbumKey, messageID domain.MessageID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.albums[key]
	if !ok {
		e = &entry{}
	}
	e.messageIDs = append(e.messageIDs, messageID)
	e.lastReceived = time.Now()
	s.albums[key] = e
	return !ok
}

// LastReceivedAge возвращает время с момента последнего сообщения в альбоме.
func (s *Service) LastReceivedAge(key domain.MediaAlbumKey) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.albums[key]; ok {
		return time.Since(e.lastReceived)
	}
	return 0
}

// PopMessages извлекает и удаляет все сообщения альбома.
func (s *Service) PopMessages(key domain.MediaAlbumKey) []domain.MessageID {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.albums[key]
	if !ok {
		return nil
	}
	ids := e.messageIDs
	delete(s.albums, key)
	return ids
}

// MakeKey формирует ключ альбома из ID правила и ID медиа-группы.
func MakeKey(ruleID domain.ForwardRuleID, mediaAlbumID int64) domain.MediaAlbumKey {
	return fmt.Sprintf("%s:%d", ruleID, mediaAlbumID)
}
