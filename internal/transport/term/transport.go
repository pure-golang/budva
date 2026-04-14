package term

import (
	"context"
	"log/slog"
)

// Transport реализует терминальный интерфейс.
type Transport struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр терминального транспорта.
func New() *Transport {
	return &Transport{
		logger: slog.Default().With("module", "transport.term"),
	}
}

// Run запускает терминальный интерфейс.
func (t *Transport) Run(ctx context.Context) error {
	t.logger.Info("Terminal transport started")
	<-ctx.Done()
	return nil
}
