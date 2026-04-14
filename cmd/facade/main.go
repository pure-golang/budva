package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	aenv "github.com/pure-golang/adapters/env"
	amiddleware "github.com/pure-golang/adapters/httpserver/middleware"
	ahttp "github.com/pure-golang/adapters/httpserver/std"
	"github.com/pure-golang/platform/monitoring"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/controller"
	"github.com/pure-golang/budva-claude/internal/repo/telegram"
	repoterm "github.com/pure-golang/budva-claude/internal/repo/term"
	"github.com/pure-golang/budva-claude/internal/service/auth"
	"github.com/pure-golang/budva-claude/internal/service/facade"
	httptransport "github.com/pure-golang/budva-claude/internal/transport/http"
	termtransport "github.com/pure-golang/budva-claude/internal/transport/term"
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

	// 3. Monitoring
	closeMonitoring := monitoring.InitDefault(cfg.Monitoring)
	defer func() {
		if err := closeMonitoring(); err != nil {
			slog.Default().Warn("Failed to close monitoring", slog.Any("err", err))
		}
	}()

	logger := slog.Default().With("module", "main")
	logger.Info("Starting facade")

	// 4. Repository-адаптеры
	telegramRepo := telegram.New(cfg.Telegram)
	if err := telegramRepo.Start(ctx); err != nil {
		return fmt.Errorf("telegram repo: %w", err)
	}
	defer func() {
		if err := telegramRepo.Close(); err != nil {
			logger.Warn("Failed to close telegram repo", slog.Any("err", err))
		}
	}()

	// 5. Сервисы
	authSvc := auth.New()
	_ = facade.New(telegramRepo)

	// 6. HTTP transport
	mux := http.NewServeMux()
	ctrl := controller.New()
	ctrl.EnrichRoutes(mux)
	httpTransport := httptransport.New(authSvc)
	httpTransport.EnrichRoutes(mux)

	handler := amiddleware.Chain(mux, amiddleware.Monitoring(), amiddleware.Recovery)

	httpServer := ahttp.NewDefault(cfg.HTTPServer, handler)

	// 7. Terminal transport
	termRepo := repoterm.New(os.Stdin, os.Stdout, int(os.Stdin.Fd())) //nolint:gosec // fd всегда 0 для stdin
	termTransport := termtransport.New(authSvc, telegramRepo, termRepo, cfg.Telegram.Phone)
	go func() {
		if err := termTransport.Run(ctx, cancel); err != nil {
			logger.Error("Terminal transport error", slog.Any("err", err))
		}
	}()

	// 8. Запуск HTTP-сервера
	go func() {
		logger.Info("HTTP server starting", slog.Int("port", cfg.HTTPServer.Port))
		httpServer.Run()
	}()

	logger.Info("Facade started, waiting for shutdown signal")
	<-ctx.Done()
	logger.Info("Shutting down facade")

	if err := httpServer.Shutdown(); err != nil {
		logger.Error("HTTP server shutdown error", slog.Any("err", err))
	}

	return nil
}
