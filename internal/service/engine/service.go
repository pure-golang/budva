package engine

import (
	"log/slog"
)

// Service диспетчеризирует обновления Telegram в обработчики.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр движка.
func New(logger *slog.Logger) *Service {
	return &Service{
		logger: logger.With("module", "service.engine"),
	}
}
