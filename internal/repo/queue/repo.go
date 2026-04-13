package queue

import (
	"container/list"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Repo реализует in-process FIFO-очередь с последовательным выполнением задач.
type Repo struct {
	logger *slog.Logger
	queue  *list.List
	mu     sync.RWMutex
}

// New создаёт новый экземпляр очереди задач.
func New(logger *slog.Logger) *Repo {
	return &Repo{
		logger: logger.With("module", "repo.queue"),
		queue:  list.New(),
	}
}

// StartContext запускает горутину обработки очереди.
func (r *Repo) StartContext(ctx context.Context) error {
	go r.run(ctx)
	return nil
}

// Close останавливает очередь.
func (r *Repo) Close() error {
	return nil
}

// Add добавляет задачу в конец очереди.
func (r *Repo) Add(fn func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queue.PushBack(fn)
}

// Len возвращает количество задач в очереди.
func (r *Repo) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.queue.Len()
}

func (r *Repo) run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.processQueue()
		}
	}
}

func (r *Repo) processQueue() {
	r.mu.Lock()
	defer r.mu.Unlock()

	front := r.queue.Front()
	if front == nil {
		return
	}

	fn := front.Value.(func())
	r.queue.Remove(front)
	r.executeTask(fn)
}

func (r *Repo) executeTask(fn func()) {
	defer func() {
		if rec := recover(); rec != nil {
			r.logger.Error("Task panicked", "error", fmt.Sprintf("%v", rec))
		}
	}()
	fn()
}
