package facade

import (
	"log/slog"
)

// Service реализует фасад для внешнего доступа к Telegram.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр фасада.
func New() *Service {
	return &Service{
		logger: slog.Default().With("module", "service.facade"),
	}
}
