package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
)

// sendMessageTimeout — максимальное время ожидания permanent ID после SendMessage.
// Запас на FLOOD_WAIT и rate-limiting тестового DC.
const sendMessageTimeout = 60 * time.Second

// sendResult — результат ожидания permanent ID у конкретного tmp message id.
// Ровно одно из полей msg/err заполнено.
type sendResult struct {
	msg *client.Message
	err error
}

// Repo — TDLib-адаптер.
//
// Разделение поверхности пакета:
//   - что есть в go-tdlib → обёртки в client_adapter.go (`r.tdClient.X(req)`)
//   - чего нет в go-tdlib → в этом файле: lifecycle, авторизация, композитные
//     операции (например, SendMessageAndWait) и каналы (Updates, AuthStates,
//     ClientDone).
type Repo struct {
	clientAdapter
	logger     *slog.Logger
	cfg        config.TelegramConfig
	mu         sync.RWMutex
	phoneCh    chan string
	codeCh     chan string
	passwordCh chan string
	clientDone chan struct{}
	updates    chan client.Type
	authStates chan domain.AuthStateEvent

	// pendingSends хранит канал-результат для каждого tmp message id, ожидающего
	// permanent ID. Единственный listener в listenUpdates доставляет результат
	// в канал и одновременно пропускает update в общий канал r.updates.
	//
	// Такой подход заменяет per-call GetListener()/Close() — go-tdlib v0.7.6
	// имеет гонку в Listener.Close() (panic: send on closed channel).
	pendingSends sync.Map // map[int64]chan sendResult
}

// New создаёт Telegram-репозиторий.
func New(cfg config.TelegramConfig) *Repo {
	return &Repo{
		logger:     slog.Default().With("module", "repo.telegram"),
		cfg:        cfg,
		clientDone: make(chan struct{}),
		updates:    make(chan client.Type, 100),
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
	r.clientAdapter = nil
	return nil
}

// AuthStates возвращает канал событий авторизации.
func (r *Repo) AuthStates() <-chan domain.AuthStateEvent {
	return r.authStates
}

// ClientDone возвращает канал, закрывающийся после инициализации TDLib-клиента.
func (r *Repo) ClientDone() <-chan struct{} {
	return r.clientDone
}

// Updates возвращает канал сырых TDLib-обновлений, прошедших фильтр.
// Потребитель сам делает type switch по `client.Type` и при необходимости
// резолвит edit-update через GetMessage.
func (r *Repo) Updates() <-chan client.Type {
	return r.updates
}

// --- Авторизация ---

// SubmitPhone отправляет номер телефона для авторизации.
func (r *Repo) SubmitPhone(_ context.Context, phone string) error {
	r.logger.Info("Phone submitted", slog.String("phone", domain.MaskPhoneNumber(phone)))
	r.mu.RLock()
	ch := r.phoneCh
	r.mu.RUnlock()
	ch <- phone
	return nil
}

// SubmitCode отправляет код подтверждения.
func (r *Repo) SubmitCode(_ context.Context, code string) error {
	r.logger.Info("Code submitted")
	r.mu.RLock()
	ch := r.codeCh
	r.mu.RUnlock()
	ch <- code
	return nil
}

// SubmitPassword отправляет пароль двухфакторной аутентификации.
func (r *Repo) SubmitPassword(_ context.Context, password string) error {
	r.logger.Info("Password submitted")
	r.mu.RLock()
	ch := r.passwordCh
	r.mu.RUnlock()
	ch <- password
	return nil
}

// CleanUp удаляет локальные данные TDLib (БД и файлы).
// После logout БД остаётся в состоянии LoggingOut, которое go-tdlib
// не умеет обрабатывать при следующем запуске. Удаление гарантирует
// чистый старт с WaitPhoneNumber.
func (r *Repo) CleanUp() {
	if r.cfg.DatabaseDirectory != "" {
		if err := os.RemoveAll(r.cfg.DatabaseDirectory); err != nil {
			r.logger.Warn("Failed to remove TDLib database directory", slog.String("path", r.cfg.DatabaseDirectory), slog.Any("err", err))
		}
	}
	if r.cfg.FilesDirectory != "" {
		if err := os.RemoveAll(r.cfg.FilesDirectory); err != nil {
			r.logger.Warn("Failed to remove TDLib files directory", slog.String("path", r.cfg.FilesDirectory), slog.Any("err", err))
		}
	}
}

// --- Композитные операции без прямого аналога в go-tdlib ---

// sendFloodWaitRetries — максимальное число повторных попыток при FLOOD_WAIT.
const sendFloodWaitRetries = 2

// floodWaitRe парсит секунды из TDLib-сообщения «Too Many Requests: retry after N».
var floodWaitRe = regexp.MustCompile(`retry after (\d+)`)

// SendMessageAndWait отправляет сообщение и ждёт присвоения permanent ID.
// TDLib возвращает temporary ID из SendMessage; permanent ID приходит
// асинхронно через UpdateMessageSendSucceeded. Метод блокирует до получения
// permanent ID, ctx.Done() или таймаута (sendMessageTimeout).
//
// При FLOOD_WAIT (TDLib error code 429) метод ждёт указанный retry-after
// и повторяет SendMessage до sendFloodWaitRetries раз.
func (r *Repo) SendMessageAndWait(ctx context.Context, req *client.SendMessageRequest) (*client.Message, error) {
	for attempt := 0; ; attempt++ {
		msg, wait, err := r.sendMessageAndWaitOnce(ctx, req)
		if err == nil {
			return msg, nil
		}
		if wait == 0 || attempt >= sendFloodWaitRetries {
			return nil, err
		}
		r.logger.Warn("FLOOD_WAIT, sleeping before retry",
			slog.Int64("chat_id", req.ChatId),
			slog.Duration("wait", wait),
			slog.Int("attempt", attempt+1),
		)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// sendMessageAndWaitOnce — одна попытка отправить и дождаться permanent ID.
// Возвращает (message, 0, nil) при успехе; (nil, waitDuration, err) при FLOOD_WAIT
// (значение wait — время ожидания из TDLib-ответа); (nil, 0, err) при иной ошибке.
func (r *Repo) sendMessageAndWaitOnce(ctx context.Context, req *client.SendMessageRequest) (*client.Message, time.Duration, error) {
	// Подписка идёт ЧЕРЕЗ ОБЩИЙ listener (listenUpdates): заводим одноразовый
	// канал-результат, слушает его listenUpdates при получении Update*Send*.
	// Так исключена гонка Listener.Close() в go-tdlib v0.7.6.
	resultCh := make(chan sendResult, 1)

	tmp, err := r.SendMessage(req)
	if err != nil {
		if wait := parseFloodWait(err.Error()); wait > 0 {
			return nil, wait, err
		}
		return nil, 0, err
	}
	tmpID := tmp.Id
	r.pendingSends.Store(tmpID, resultCh)
	defer r.pendingSends.Delete(tmpID)

	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	case <-time.After(sendMessageTimeout):
		return nil, 0, fmt.Errorf("timeout waiting for permanent ID of message %d in chat %d", tmpID, req.ChatId)
	case res := <-resultCh:
		if res.err == nil {
			r.logger.Debug("Permanent ID received",
				slog.Int64("chat_id", req.ChatId),
				slog.Int64("tmp_id", tmpID),
				slog.Int64("permanent_id", res.msg.Id),
			)
			return res.msg, 0, nil
		}
		if wait := parseFloodWait(res.err.Error()); wait > 0 {
			return nil, wait, res.err
		}
		return nil, 0, res.err
	}
}

// parseFloodWait извлекает длительность ожидания из текста «retry after N».
// Возвращает 0, если паттерн не найден.
func parseFloodWait(s string) time.Duration {
	m := floodWaitRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	secs, err := strconv.Atoi(m[1])
	if err != nil || secs <= 0 {
		return 0
	}
	// Небольшой запас (500 мс) поверх указанного TDLib значения.
	return time.Duration(secs)*time.Second + 500*time.Millisecond
}

// --- Internal lifecycle ---

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

	r.clientAdapter = tdlibClient
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
			// tdClient намеренно не обнуляется при Closed: обёртки clientAdapter
			// читают r.tdClient без mu, а добавлять мьютекс на каждый вызов
			// ради cleanup на выходе неоправданно. Close() очищает поле,
			// а при AuthorizationStateClosed процесс либо завершится, либо
			// auth-сервис перезапустит lifecycle.
			event, relevant := mapTDLibState(state)
			if relevant {
				r.authStates <- event
			}
		}
	}
}

func (r *Repo) listenUpdates(ctx context.Context) {
	listener := r.GetListener()
	defer listener.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case typ, ok := <-listener.Updates:
			if !ok {
				return
			}
			// Сначала доставляем результат подписчикам SendMessageAndWait,
			// затем — если update релевантен — кладём в общий канал.
			r.dispatchSendResult(typ)
			if !isRelevantUpdate(typ) {
				continue
			}
			r.updates <- typ
		}
	}
}

// dispatchSendResult доставляет permanent ID / send-error подписчику
// SendMessageAndWait по tmp message id. No-op, если подписчика нет.
func (r *Repo) dispatchSendResult(typ client.Type) {
	switch u := typ.(type) {
	case *client.UpdateMessageSendSucceeded:
		r.deliverSendResult(u.OldMessageId, sendResult{msg: u.Message})
	case *client.UpdateMessageSendFailed:
		code := int32(0)
		msg := "unknown"
		if u.Error != nil {
			code = u.Error.Code
			msg = u.Error.Message
		}
		r.deliverSendResult(u.OldMessageId, sendResult{
			err: fmt.Errorf("send failed: code=%d %s", code, msg),
		})
	}
}

func (r *Repo) deliverSendResult(tmpID int64, res sendResult) {
	chAny, ok := r.pendingSends.LoadAndDelete(tmpID)
	if !ok {
		return
	}
	ch, ok := chAny.(chan sendResult)
	if !ok {
		return
	}
	// Буфер канала = 1 и LoadAndDelete гарантирует ровно одну доставку,
	// поэтому send никогда не блокируется.
	ch <- res
}

// isRelevantUpdate отсеивает TDLib-типы, которые не интересны потребителям.
// Фильтр здесь, а не у потребителя, чтобы не засорять канал.
func isRelevantUpdate(typ client.Type) bool {
	switch u := typ.(type) {
	case *client.UpdateNewMessage,
		*client.UpdateMessageSendSucceeded,
		*client.UpdateMessageEdited:
		return true
	case *client.UpdateDeleteMessages:
		return u.IsPermanent
	default:
		return false
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
		// WaitTdlibParameters, Closing, LoggingOut и т.д. — обрабатываются внутри go-tdlib.
		return domain.AuthStateEvent{}, false
	}
}
