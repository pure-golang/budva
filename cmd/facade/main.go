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
	agrpc "github.com/pure-golang/adapters/grpc/std"
	amiddleware "github.com/pure-golang/adapters/httpserver/middleware"
	ahttp "github.com/pure-golang/adapters/httpserver/std"
	"github.com/pure-golang/platform/monitoring"
	"google.golang.org/grpc"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/controller"
	"github.com/pure-golang/budva-claude/internal/repo/telegram"
	"github.com/pure-golang/budva-claude/internal/repo/term"
	"github.com/pure-golang/budva-claude/internal/service/auth"
	"github.com/pure-golang/budva-claude/internal/service/facade"
	tgrpc "github.com/pure-golang/budva-claude/internal/transport/grpc"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
	thttp "github.com/pure-golang/budva-claude/internal/transport/http"
	"github.com/pure-golang/budva-claude/internal/transport/http/resolvers"
	tterm "github.com/pure-golang/budva-claude/internal/transport/term"
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
	logger := slog.Default().With("module", "main")
	defer func() {
		if err := closeMonitoring(); err != nil {
			logger.Error("Failed to close monitoring", slog.Any("err", err))
		}
	}()

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
	authService := auth.New(telegramRepo)
	authService.Start(ctx)

	facadeService := facade.New(telegramRepo)

	// 6. HTTP transport
	mux := http.NewServeMux()
	ctrl := controller.New()
	ctrl.EnrichRoutes(mux)
	gqlResolver := resolvers.New(facadeService)
	httpTransport := thttp.New(authService, gqlResolver)
	httpTransport.EnrichRoutes(mux)

	handler := amiddleware.Chain(mux, amiddleware.Monitoring("/graphql", "/api/*"), amiddleware.Recovery)

	httpServer := ahttp.NewDefault(cfg.HTTPServer, handler)

	// 7. gRPC transport
	grpcServer := agrpc.NewDefault(cfg.GRPCServer, func(s *grpc.Server) {
		grpcTransport := tgrpc.New(facadeService)
		pb.RegisterFacadeGRPCServer(s, grpcTransport)
	})

	// 8. Terminal transport
	termRepo := term.New(os.Stdin, os.Stdout, int(os.Stdin.Fd())) //nolint:gosec // fd всегда 0 для stdin
	termTransport := tterm.New(authService, telegramRepo, termRepo, cfg.Telegram.Phone)
	go func() {
		if err := termTransport.Run(ctx, cancel); err != nil {
			logger.Error("Terminal transport error", slog.Any("err", err))
		}
	}()

	// 9. Запуск серверов
	go func() {
		logger.Info("HTTP server starting", slog.Int("port", cfg.HTTPServer.Port))
		httpServer.Run()
	}()

	go func() {
		grpcServer.Run()
	}()

	logger.Info("Facade started, waiting for shutdown signal")
	<-ctx.Done()
	logger.Info("Shutting down facade")

	if err := grpcServer.Shutdown(); err != nil {
		logger.Error("gRPC server shutdown error", slog.Any("err", err))
	}

	if err := httpServer.Shutdown(); err != nil {
		logger.Error("HTTP server shutdown error", slog.Any("err", err))
	}

	return nil
}
