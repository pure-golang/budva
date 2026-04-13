package forwarder

import (
	"log/slog"
)

// Service отправляет и копирует сообщения в целевые чаты.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр сервиса пересылки.
func New(logger *slog.Logger) *Service {
	return &Service{
		logger: logger.With("module", "service.forwarder"),
	}
}
