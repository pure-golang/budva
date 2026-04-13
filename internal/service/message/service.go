package message

import (
	"log/slog"
)

// Service извлекает и формирует контент сообщений.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр сервиса сообщений.
func New(logger *slog.Logger) *Service {
	return &Service{
		logger: logger.With("module", "service.message"),
	}
}
