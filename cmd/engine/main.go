package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	aenv "github.com/pure-golang/adapters/env"
	"github.com/pure-golang/platform/monitoring"

	"github.com/pure-golang/budva/internal/config"
	"github.com/pure-golang/budva/internal/handler"
	"github.com/pure-golang/budva/internal/repo/queue"
	"github.com/pure-golang/budva/internal/repo/ruleset"
	"github.com/pure-golang/budva/internal/repo/state"
	"github.com/pure-golang/budva/internal/repo/telegram"
	"github.com/pure-golang/budva/internal/service/album"
	"github.com/pure-golang/budva/internal/service/auth"
	"github.com/pure-golang/budva/internal/service/dedup"
	"github.com/pure-golang/budva/internal/service/engine"
	"github.com/pure-golang/budva/internal/service/filters"
	"github.com/pure-golang/budva/internal/service/forwarder"
	"github.com/pure-golang/budva/internal/service/limiter"
	"github.com/pure-golang/budva/internal/service/loader"
	"github.com/pure-golang/budva/internal/service/message"
	"github.com/pure-golang/budva/internal/service/transform"
	termtransport "github.com/pure-golang/budva/internal/transport/term"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// 1. Контекст с обработкой сигналов
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 2. Конфигурация
	var cfg config.Config
	if err := aenv.InitConfig(&cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// 3. Monitoring (logger, tracing, metrics)
	closeMonitoring := monitoring.InitDefault(cfg.Monitoring)
	defer closeMonitoring()

	logger := slog.Default().With("module", "main")
	logger.Info("Starting engine")

	// 4. Repository-адаптеры
	stateRepo := state.New(cfg.Storage, slog.Default())
	if err := stateRepo.Start(ctx); err != nil {
		return fmt.Errorf("state repo: %w", err)
	}
	defer stateRepo.Close()

	rulesetRepo := ruleset.New(cfg.Ruleset, slog.Default())

	telegramRepo := telegram.New(cfg.Telegram, slog.Default())
	if err := telegramRepo.Start(ctx); err != nil {
		return fmt.Errorf("telegram repo: %w", err)
	}
	defer telegramRepo.Close()

	queueRepo := queue.New(slog.Default())
	if err := queueRepo.StartContext(ctx); err != nil {
		return fmt.Errorf("queue repo: %w", err)
	}
	defer queueRepo.Close()

	// 5. Сервисы
	_ = auth.New(slog.Default())
	_ = message.New(slog.Default())
	_ = transform.New(telegramRepo, slog.Default())
	_ = filters.New(slog.Default())
	_ = album.New(slog.Default())
	_ = forwarder.New(slog.Default())
	_ = dedup.NewTracker(nil)
	_ = limiter.New(slog.Default())
	_ = loader.New(slog.Default())
	_ = engine.New(slog.Default())

	// 6. Handlers
	_ = handler.New(slog.Default())

	// 7. Ruleset
	_, err := rulesetRepo.Load()
	if err != nil {
		logger.Warn("Failed to load ruleset", "error", err)
	}

	// 8. Terminal transport
	termTransport := termtransport.New(slog.Default())
	go termTransport.Run(ctx)

	logger.Info("Engine started, waiting for shutdown signal")
	<-ctx.Done()
	logger.Info("Shutting down engine")

	return nil
}
