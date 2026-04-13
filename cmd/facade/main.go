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
	ahttp "github.com/pure-golang/adapters/httpserver/std"
	amiddleware "github.com/pure-golang/adapters/httpserver/middleware"
	"github.com/pure-golang/platform/monitoring"

	"github.com/pure-golang/budva/internal/config"
	"github.com/pure-golang/budva/internal/controller"
	"github.com/pure-golang/budva/internal/repo/telegram"
	"github.com/pure-golang/budva/internal/service/auth"
	"github.com/pure-golang/budva/internal/service/facade"
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

	// 3. Monitoring
	closeMonitoring := monitoring.InitDefault(cfg.Monitoring)
	defer closeMonitoring()

	logger := slog.Default().With("module", "main")
	logger.Info("Starting facade")

	// 4. Repository-адаптеры
	telegramRepo := telegram.New(cfg.Telegram, slog.Default())
	if err := telegramRepo.Start(ctx); err != nil {
		return fmt.Errorf("telegram repo: %w", err)
	}
	defer telegramRepo.Close()

	// 5. Сервисы
	_ = auth.New(slog.Default())
	_ = facade.New(slog.Default())

	// 6. HTTP transport
	mux := http.NewServeMux()
	ctrl := controller.New()
	ctrl.EnrichRoutes(mux)

	handler := amiddleware.Chain(mux, amiddleware.Monitoring(), amiddleware.Recovery)

	httpServer := ahttp.NewDefault(cfg.HTTPServer, handler)

	// 7. Terminal transport
	termTransport := termtransport.New(slog.Default())
	go termTransport.Run(ctx)

	// 8. Запуск HTTP-сервера
	go func() {
		logger.Info("HTTP server starting", "port", cfg.HTTPServer.Port)
		httpServer.Run()
	}()

	logger.Info("Facade started, waiting for shutdown signal")
	<-ctx.Done()
	logger.Info("Shutting down facade")

	if err := httpServer.Shutdown(); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	return nil
}
