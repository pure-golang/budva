package loader

import (
	"log/slog"
)

// Service загружает правила пересылки и прогревает чаты.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр загрузчика.
func New() *Service {
	return &Service{
		logger: slog.Default().With("module", "service.loader"),
	}
}
