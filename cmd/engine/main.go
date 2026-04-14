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

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/handler"
	"github.com/pure-golang/budva-claude/internal/repo/queue"
	"github.com/pure-golang/budva-claude/internal/repo/ruleset"
	"github.com/pure-golang/budva-claude/internal/repo/state"
	"github.com/pure-golang/budva-claude/internal/repo/telegram"
	repoterm "github.com/pure-golang/budva-claude/internal/repo/term"
	"github.com/pure-golang/budva-claude/internal/service/album"
	"github.com/pure-golang/budva-claude/internal/service/auth"
	"github.com/pure-golang/budva-claude/internal/service/dedup"
	"github.com/pure-golang/budva-claude/internal/service/filters"
	"github.com/pure-golang/budva-claude/internal/service/message"
	"github.com/pure-golang/budva-claude/internal/service/transform"
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

	// 3. Monitoring (logger, tracing, metrics)
	closeMonitoring := monitoring.InitDefault(cfg.Monitoring)
	defer func() {
		if err := closeMonitoring(); err != nil {
			slog.Default().Warn("Failed to close monitoring", slog.Any("err", err))
		}
	}()

	logger := slog.Default().With("module", "main")
	logger.Info("Starting engine")

	// 4. Repository-адаптеры
	stateRepo := state.New(cfg.Storage)
	if err := stateRepo.Start(ctx); err != nil {
		return fmt.Errorf("state repo: %w", err)
	}
	defer func() {
		if err := stateRepo.Close(); err != nil {
			logger.Warn("Failed to close state repo", slog.Any("err", err))
		}
	}()

	rulesetRepo := ruleset.New(cfg.Ruleset)

	telegramRepo := telegram.New(cfg.Telegram)
	if err := telegramRepo.Start(ctx); err != nil {
		return fmt.Errorf("telegram repo: %w", err)
	}
	defer func() {
		if err := telegramRepo.Close(); err != nil {
			logger.Warn("Failed to close telegram repo", slog.Any("err", err))
		}
	}()

	queueRepo := queue.New()
	if err := queueRepo.StartContext(ctx); err != nil {
		return fmt.Errorf("queue repo: %w", err)
	}
	defer func() {
		if err := queueRepo.Close(); err != nil {
			logger.Warn("Failed to close queue repo", slog.Any("err", err))
		}
	}()

	// 5. Сервисы
	authSvc := auth.New()
	messageSvc := message.New()
	transformSvc := transform.New(telegramRepo, stateRepo)
	filtersSvc := filters.New()
	albumSvc := album.New()

	// 6. Handler
	h := handler.New(
		telegramRepo,
		stateRepo,
		messageSvc,
		filtersSvc,
		transformSvc,
		albumSvc,
		queueRepo,
		func(dsts []domain.ChatID) handler.DedupTracker {
			return dedup.NewTracker(dsts)
		},
	)

	// 7. Ruleset
	rs, err := rulesetRepo.Load()
	if err != nil {
		logger.Warn("Failed to load ruleset", slog.Any("err", err))
	} else {
		h.SetRuleSet(rs)
	}

	// 8. Watcher для hot-reload
	if err := rulesetRepo.WatchContext(ctx, func() {
		newRS, loadErr := rulesetRepo.Load()
		if loadErr != nil {
			logger.Error("Failed to reload ruleset", slog.Any("err", loadErr))
			return
		}
		h.SetRuleSet(newRS)
		logger.Info("Ruleset reloaded")
	}); err != nil {
		logger.Warn("Failed to watch ruleset", slog.Any("err", err))
	}
	defer func() {
		if err := rulesetRepo.Close(); err != nil {
			logger.Warn("Failed to close ruleset repo", slog.Any("err", err))
		}
	}()

	// 9. Update dispatcher
	go func() {
		<-telegramRepo.ClientDone()
		for {
			select {
			case <-ctx.Done():
				return
			case update, ok := <-telegramRepo.Updates():
				if !ok {
					return
				}
				switch update.Type {
				case domain.UpdateNewMessage:
					h.OnNewMessage(ctx, update.Message)
				case domain.UpdateMessageEdited:
					h.OnEditedMessage(ctx, update.Message)
				case domain.UpdateDeleteMessages:
					h.OnDeletedMessages(ctx, update.ChatID, update.MessageIDs, update.IsPermanent)
				case domain.UpdateMessageSendSucceeded:
					h.OnMessageSendSucceeded(update.Message.ChatID, update.OldMessageID, update.Message.ID)
				}
			}
		}
	}()

	// 10. Terminal transport
	termRepo := repoterm.New(os.Stdin, os.Stdout, int(os.Stdin.Fd())) //nolint:gosec // fd всегда 0 для stdin
	termTransport := termtransport.New(authSvc, telegramRepo, termRepo, cfg.Telegram.Phone)
	go func() {
		if err := termTransport.Run(ctx, cancel); err != nil {
			logger.Error("Terminal transport error", slog.Any("err", err))
		}
	}()

	logger.Info("Engine started, waiting for shutdown signal")
	<-ctx.Done()
	logger.Info("Shutting down engine")

	return nil
}
