package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
)

// Repo реализует взаимодействие с Telegram через TDLib.
type Repo struct {
	logger     *slog.Logger
	cfg        config.TelegramConfig
	tdClient   *client.Client
	mu         sync.RWMutex
	phoneCh    chan string
	codeCh     chan string
	passwordCh chan string
	clientDone chan struct{}
	updates    chan domain.Update
	authStates chan domain.AuthStateEvent
}

// New создаёт новый экземпляр Telegram-репозитория.
func New(cfg config.TelegramConfig) *Repo {
	return &Repo{
		logger:     slog.Default().With("module", "repo.telegram"),
		cfg:        cfg,
		clientDone: make(chan struct{}),
		updates:    make(chan domain.Update, 100),
		authStates: make(chan domain.AuthStateEvent, 10),
	}
}

// Start инициализирует TDLib-клиент и запускает авторизацию.
func (r *Repo) Start(ctx context.Context) error {
	if err := r.setupClientLog(); err != nil {
		return err
	}
	go r.runAuthLoop(ctx)
	return nil
}

// Close завершает TDLib-сессию.
// go-tdlib v0.7.6 имеет гонку в глобальном receiver при Close,
// вызывающую SIGABRT из C++ runtime. Пропускаем явный Close —
// TDLib корректно завершает сессию при выходе процесса.
func (r *Repo) Close() error {
	r.tdClient = nil
	return nil
}

// AuthStates возвращает канал событий авторизации.
func (r *Repo) AuthStates() <-chan domain.AuthStateEvent {
	return r.authStates
}

// ClientDone возвращает канал, закрывающийся после авторизации.
func (r *Repo) ClientDone() <-chan struct{} {
	return r.clientDone
}

// Updates возвращает канал обновлений.
func (r *Repo) Updates() <-chan domain.Update {
	return r.updates
}

func (r *Repo) setupClientLog() error {
	logDir := r.cfg.LogDirectory
	if logDir == "" {
		logDir = ".data/tdlib-logs"
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create tdlib log dir: %w", err)
	}

	_, err := client.SetLogStream(&client.SetLogStreamRequest{
		LogStream: &client.LogStreamFile{
			Path:           filepath.Join(logDir, "telegram.log"),
			MaxFileSize:    r.cfg.LogMaxFileSize * 1024 * 1024,
			RedirectStderr: false,
		},
	})
	if err != nil {
		return fmt.Errorf("set tdlib log stream: %w", err)
	}

	_, err = client.SetLogVerbosityLevel(&client.SetLogVerbosityLevelRequest{
		NewVerbosityLevel: r.cfg.LogVerbosityLevel,
	})
	if err != nil {
		return fmt.Errorf("set tdlib log verbosity: %w", err)
	}

	return nil
}

func (r *Repo) createTdlibParameters() *client.SetTdlibParametersRequest {
	return &client.SetTdlibParametersRequest{
		UseTestDc:           false,
		DatabaseDirectory:   r.cfg.DatabaseDirectory,
		FilesDirectory:      r.cfg.FilesDirectory,
		UseFileDatabase:     r.cfg.UseFileDatabase,
		UseChatInfoDatabase: r.cfg.UseChatInfoDatabase,
		UseMessageDatabase:  r.cfg.UseMessageDatabase,
		UseSecretChats:      r.cfg.UseSecretChats,
		ApiId:               r.cfg.APIID,
		ApiHash:             r.cfg.APIHash,
		SystemLanguageCode:  r.cfg.SystemLanguageCode,
		DeviceModel:         r.cfg.DeviceModel,
		SystemVersion:       r.cfg.SystemVersion,
		ApplicationVersion:  r.cfg.ApplicationVersion,
	}
}

func (r *Repo) runAuthLoop(ctx context.Context) {
	authorizer := client.ClientAuthorizer(r.createTdlibParameters())

	r.mu.Lock()
	r.phoneCh = authorizer.PhoneNumber
	r.codeCh = authorizer.Code
	r.passwordCh = authorizer.Password
	r.mu.Unlock()

	go r.listenAuthStates(ctx, authorizer.State)

	tdlibClient, err := client.NewClient(authorizer)
	if err != nil {
		r.logger.Error("Failed to create TDLib client", slog.Any("err", err))
		return
	}

	r.tdClient = tdlibClient
	close(r.clientDone)

	go r.listenUpdates(ctx)
}

func (r *Repo) listenAuthStates(ctx context.Context, states <-chan client.AuthorizationState) {
	for {
		select {
		case <-ctx.Done():
			return
		case state, ok := <-states:
			if !ok {
				return
			}
			event, relevant := mapTDLibState(state)
			if relevant {
				r.authStates <- event
			}
		}
	}
}

func (r *Repo) listenUpdates(ctx context.Context) {
	listener := r.tdClient.GetListener()
	defer listener.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case typ, ok := <-listener.Updates:
			if !ok {
				return
			}
			if upd, mapped := r.mapUpdate(typ); mapped {
				r.updates <- upd
			}
		}
	}
}

func (r *Repo) mapUpdate(typ client.Type) (domain.Update, bool) {
	switch u := typ.(type) {
	case *client.UpdateNewMessage:
		msg := mapMessage(u.Message)
		return domain.Update{Type: domain.UpdateNewMessage, Message: msg}, true

	case *client.UpdateMessageSendSucceeded:
		msg := mapMessage(u.Message)
		return domain.Update{
			Type:         domain.UpdateMessageSendSucceeded,
			Message:      msg,
			OldMessageID: u.OldMessageId,
		}, true

	case *client.UpdateDeleteMessages:
		if !u.IsPermanent {
			return domain.Update{}, false
		}
		return domain.Update{
			Type:        domain.UpdateDeleteMessages,
			ChatID:      u.ChatId,
			MessageIDs:  u.MessageIds,
			IsPermanent: u.IsPermanent,
		}, true

	default:
		return domain.Update{}, false
	}
}

// mapTDLibState конвертирует TDLib AuthorizationState в domain.AuthStateEvent.
// Возвращает (event, true) для состояний, релевантных auth.Service.
// Внутренние состояния TDLib (WaitTdlibParameters, Closing и т.д.) пропускаются.
func mapTDLibState(state client.AuthorizationState) (domain.AuthStateEvent, bool) {
	switch s := state.(type) {
	case *client.AuthorizationStateWaitPhoneNumber:
		return domain.AuthStateEvent{State: domain.AuthStateWaitPhone}, true
	case *client.AuthorizationStateWaitCode:
		return domain.AuthStateEvent{State: domain.AuthStateWaitCode}, true
	case *client.AuthorizationStateWaitPassword:
		return domain.AuthStateEvent{
			State: domain.AuthStateWaitPassword,
			Extra: &domain.WaitPasswordState{PasswordHint: s.PasswordHint},
		}, true
	case *client.AuthorizationStateReady:
		return domain.AuthStateEvent{State: domain.AuthStateReady}, true
	case *client.AuthorizationStateClosed:
		return domain.AuthStateEvent{State: domain.AuthStateClosed}, true
	default:
		// WaitTdlibParameters, Closing, LoggingOut и т.д. — обрабатываются внутри go-tdlib
		return domain.AuthStateEvent{}, false
	}
}
