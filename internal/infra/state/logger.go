package state

import (
	"fmt"
	"log/slog"
)

// badgerLogger адаптирует *slog.Logger к интерфейсу badger.Logger.
type badgerLogger struct {
	logger *slog.Logger
}

func newBadgerLogger(logger *slog.Logger) *badgerLogger {
	return &badgerLogger{logger: logger.WithGroup("badger")}
}

func (l *badgerLogger) Errorf(format string, args ...any) {
	l.logger.Error(fmt.Sprintf(format, args...))
}

func (l *badgerLogger) Warningf(format string, args ...any) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}

func (l *badgerLogger) Infof(format string, args ...any) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

func (l *badgerLogger) Debugf(format string, args ...any) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}
