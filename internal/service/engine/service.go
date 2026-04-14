package engine

import (
	"log/slog"
)

// Service диспетчеризирует обновления Telegram в обработчики.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр движка.
func New() *Service {
	return &Service{
		logger: slog.Default().With("module", "service.engine"),
	}
}
