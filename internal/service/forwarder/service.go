package forwarder

import (
	"log/slog"
)

// Service отправляет и копирует сообщения в целевые чаты.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр сервиса пересылки.
func New() *Service {
	return &Service{
		logger: slog.Default().With("module", "service.forwarder"),
	}
}
