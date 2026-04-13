package loader

import (
	"log/slog"
)

// Service загружает правила пересылки и прогревает чаты.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр загрузчика.
func New(logger *slog.Logger) *Service {
	return &Service{
		logger: logger.With("module", "service.loader"),
	}
}
