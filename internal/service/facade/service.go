package facade

import (
	"log/slog"
)

// Service реализует фасад для внешнего доступа к Telegram.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр фасада.
func New(logger *slog.Logger) *Service {
	return &Service{
		logger: logger.With("module", "service.facade"),
	}
}
