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
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/handler"
	"github.com/pure-golang/budva-claude/internal/repo/queue"
	"github.com/pure-golang/budva-claude/internal/repo/ruleset"
	"github.com/pure-golang/budva-claude/internal/repo/state"
	"github.com/pure-golang/budva-claude/internal/repo/telegram"
	"github.com/pure-golang/budva-claude/internal/repo/term"
	"github.com/pure-golang/budva-claude/internal/service/album"
	"github.com/pure-golang/budva-claude/internal/service/auth"
	"github.com/pure-golang/budva-claude/internal/service/dedup"
	"github.com/pure-golang/budva-claude/internal/service/filters"
	"github.com/pure-golang/budva-claude/internal/service/limiter"
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
	logger := slog.Default().With("module", "main")
	defer func() {
		if err := closeMonitoring(); err != nil {
			logger.Error("Failed to close monitoring", slog.Any("err", err))
		}
	}()

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
	authService := auth.New(telegramRepo)
	authService.Start(ctx)

	messageService := message.New()
	transformService := transform.New(telegramRepo, stateRepo)
	filterService := filters.New()
	albumService := album.New()

	limiterService := limiter.New()

	// 6. Handler
	h := handler.New(
		telegramRepo,
		stateRepo,
		messageService,
		filterService,
		transformService,
		albumService,
		queueRepo,
		limiterService,
		func(dsts []int64) handler.DedupTracker {
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
				switch u := update.(type) {
				case *client.UpdateNewMessage:
					h.OnNewMessage(ctx, u.Message)
				case *client.UpdateMessageEdited:
					// Resolve в отдельной горутине: синхронный GetMessage внутри dispatcher-а
					// блокирует приём updates, что приводит к зависанию других listener-ов TDLib.
					go func(chatID, msgID int64) {
						msg, err := telegramRepo.GetMessage(&client.GetMessageRequest{
							ChatId:    chatID,
							MessageId: msgID,
						})
						if err != nil {
							logger.Warn("Failed to get edited message",
								slog.Int64("chat_id", chatID),
								slog.Int64("message_id", msgID),
								slog.Any("err", err),
							)
							return
						}
						h.OnEditedMessage(ctx, msg)
					}(u.ChatId, u.MessageId)
				case *client.UpdateDeleteMessages:
					h.OnDeletedMessages(ctx, u.ChatId, u.MessageIds, u.IsPermanent)
				case *client.UpdateMessageSendSucceeded:
					h.OnMessageSendSucceeded(u.Message.ChatId, u.OldMessageId, u.Message.Id)
				}
			}
		}
	}()

	// 10. Terminal transport
	termRepo := term.New(os.Stdin, os.Stdout, int(os.Stdin.Fd())) //nolint:gosec // fd всегда 0 для stdin
	termTransport := termtransport.New(authService, telegramRepo, termRepo, cfg.Telegram.Phone)
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
