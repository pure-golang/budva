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
			slog.Default().Warn("Failed to close monitoring", "error", err)
		}
	}()

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
	messageSvc := message.New(slog.Default())
	transformSvc := transform.New(telegramRepo, stateRepo, slog.Default())
	filtersSvc := filters.New(slog.Default())
	albumSvc := album.New(slog.Default())

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
		slog.Default(),
	)

	// 7. Ruleset
	rs, err := rulesetRepo.Load()
	if err != nil {
		logger.Warn("Failed to load ruleset", "error", err)
	} else {
		h.SetRuleSet(rs)
	}

	// 8. Watcher для hot-reload
	if err := rulesetRepo.WatchContext(ctx, func() {
		newRS, loadErr := rulesetRepo.Load()
		if loadErr != nil {
			logger.Error("Failed to reload ruleset", "error", loadErr)
			return
		}
		h.SetRuleSet(newRS)
		logger.Info("Ruleset reloaded")
	}); err != nil {
		logger.Warn("Failed to watch ruleset", "error", err)
	}
	defer rulesetRepo.Close()

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
	termTransport := termtransport.New(slog.Default())
	go termTransport.Run(ctx)

	logger.Info("Engine started, waiting for shutdown signal")
	<-ctx.Done()
	logger.Info("Shutting down engine")

	return nil
}
