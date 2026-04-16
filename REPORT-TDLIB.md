# Phase B: план интеграции TDLib

Последняя актуализация: 2026-04-16.

## Предусловия

Phase A полностью закрыта. Вся бизнес-логика (handler, transform, filters, dedup, album, limiter, message, facade, auth) работает через абстракции `domain.*` и интерфейсы. TDLib-зависимый код локализован в `internal/repo/telegram/`.

**Текущее состояние `repo/telegram`:** fake-реализация с event-driven auth flow (SubmitPhone/Code/Password эмитят события в `authStates` канал), опциональный WaitPassword через `has2FA`, остальные методы `clientAdapter` возвращают nil/пустые значения.

**Текущая архитектура auth flow:**

```
Repo (владеет state machine):
  Start()           → эмитит AuthStateWaitPhone в authStates
  SubmitPhone()     → эмитит AuthStateWaitCode
  SubmitCode()      → эмитит AuthStateWaitPassword (has2FA) или AuthStateReady
  SubmitPassword()  → эмитит AuthStateReady, закрывает clientDone
  AuthStates()      → <-chan domain.AuthStateEvent
  ClientDone()      → <-chan struct{}

auth.Service (оркестратор):
  run() → читает AuthStates(), фильтрует Closing/Closed,
          уведомляет listeners (async), ждёт input из InputChan(),
          вызывает SubmitPhone/Code/Password
  Close() → закрывает inputChan

Transport (терминал / HTTP):
  Subscribe() на auth.Service → показывает промпты
  Отправляет input в InputChan()
```

При подключении TDLib меняется только внутренняя реализация `Repo`. Интерфейсы `clientAdapter`, `telegramRepo` (consumer-side в auth.Service) и `authService` (consumer-side в транспортах) — без изменений.

## Зависимости

```
github.com/zelenin/go-tdlib v0.7.6
```

CGO-флаги:
```
CGO_CFLAGS=-I/usr/local/include
CGO_LDFLAGS="-Wl,-rpath,/usr/local/lib -L/usr/local/lib -ltdjson -lc++"
```

Базовый Docker-образ: `tdlib-ubuntu:latest` (TDLib C++ библиотека + headers).

## Задачи

### T1. Dockerfile с TDLib

Текущий `Dockerfile` собирает без CGO. Нужно:
- Базовый образ с TDLib (или multi-stage build с `tdlib-ubuntu`)
- CGO_ENABLED=1 + CGO_CFLAGS/CGO_LDFLAGS
- Runtime-образ с `libtdjson.so` и `libc++`

**Источник:** `budva43/Dockerfile`

### T2. go-tdlib зависимость

```
go get github.com/zelenin/go-tdlib@v0.7.6
```

Добавить в `go.mod`. Убедиться что `go build` проходит с CGO_ENABLED=1.

### T3. TDLib parameters в Config

Добавить в `TelegramConfig` недостающие поля из budva43:

| Поле | Тип | Default | Описание |
|---|---|---|---|
| `UseFileDatabase` | bool | true | Файловый кеш TDLib |
| `UseChatInfoDatabase` | bool | true | Кеш информации о чатах |
| `UseMessageDatabase` | bool | true | Кеш сообщений |
| `UseSecretChats` | bool | false | Поддержка секретных чатов |
| `SystemVersion` | string | "" | Версия системы |
| `ApplicationVersion` | string | "" | Версия приложения |
| `LogDirectory` | string | ".data/tdlib-logs" | Директория логов TDLib |
| `LogMaxFileSize` | int64 | 10 | Макс размер лог-файла (MB) |

**Источник:** `budva43/repo/telegram/repo.go:63-80`

### T4. repo/telegram — TDLib клиент

Заменить fake-реализацию на реальную TDLib-интеграцию. Все изменения внутри `Repo` — интерфейс `clientAdapter` не меняется.

#### T4.1. Структура Repo

Добавить поле `client` и `authorizer`:

```go
type Repo struct {
    logger     *slog.Logger
    cfg        config.TelegramConfig
    client     *client.Client              // go-tdlib клиент (nil до авторизации)
    authorizer *client.ClientAuthorizer    // каналы для phone/code/password
    clientDone chan struct{}
    updates    chan domain.Update
    authStates chan domain.AuthStateEvent
}
```

Поле `has2FA` удаляется — TDLib сам решает, нужен ли WaitPassword.

#### T4.2. Start() — полная инициализация

```go
func (r *Repo) Start(ctx context.Context) error {
    if err := r.setupClientLog(); err != nil {
        return err
    }

    r.authorizer = client.ClientAuthorizer(r.createTdlibParameters())

    // Горутина: слушает authorizer.State, маппит в domain events
    go r.listenAuthStates(ctx)

    // Горутина: создаёт TDLib клиент (retry loop)
    go r.createClient(ctx)

    return nil
}
```

`setupClientLog()`:
- `client.SetLogStream()` → файл `cfg.LogDirectory/telegram.log`
- `client.SetLogVerbosityLevel()` → `cfg.LogVerbosityLevel`

**Источник:** `budva43/repo/telegram/repo.go:150-169`

#### T4.3. listenAuthStates() — маппинг TDLib → domain

```go
func (r *Repo) listenAuthStates(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case state, ok := <-r.authorizer.State:
            if !ok {
                r.authStates <- domain.AuthStateEvent{State: domain.AuthStateClosed}
                return
            }
            if _, isClosing := state.(*client.AuthorizationStateClosing); isClosing {
                continue
            }
            r.authStates <- mapTDLibState(state)
        }
    }
}
```

#### T4.4. createClient() — retry loop

```go
func (r *Repo) createClient(ctx context.Context) {
    for {
        tdlibClient, err := client.NewClient(r.authorizer)
        if err != nil {
            r.logger.Error("Failed to create TDLib client", slog.Any("err", err))
            continue
        }
        r.client = tdlibClient
        close(r.clientDone)
        return
    }
}
```

**Источник:** `budva43/repo/telegram/repo.go:82-113`

#### T4.5. Submit* — делегирование в authorizer

```go
func (r *Repo) SubmitPhone(_ context.Context, phone string) error {
    r.logger.Info("Phone submitted", slog.String("phone", domain.MaskPhoneNumber(phone)))
    r.authorizer.PhoneNumber <- phone
    return nil
}

func (r *Repo) SubmitCode(_ context.Context, _ string) error {
    r.logger.Info("Code submitted")
    r.authorizer.Code <- code
    return nil
}

func (r *Repo) SubmitPassword(_ context.Context, _ string) error {
    r.logger.Info("Password submitted")
    r.authorizer.Password <- password
    return nil
}
```

TDLib сам эмитит следующее состояние в `authorizer.State` → `listenAuthStates` маппит его → `authStates` канал → `auth.Service.run()` обрабатывает.

#### T4.6. Close() с sleep workaround

```go
func (r *Repo) Close() error {
    if r.client == nil {
        return nil
    }
    _, err := r.client.Close()
    r.client = nil
    time.Sleep(1 * time.Second) // TDLib signal.abort workaround
    return err
}
```

**Источник:** `budva43/repo/telegram/repo.go:116-136`

### T5. Маппинг TDLib states → domain

```go
func mapTDLibState(state client.AuthorizationState) domain.AuthStateEvent {
    switch s := state.(type) {
    case *client.AuthorizationStateWaitPhoneNumber:
        return domain.AuthStateEvent{State: domain.AuthStateWaitPhone}
    case *client.AuthorizationStateWaitCode:
        return domain.AuthStateEvent{State: domain.AuthStateWaitCode}
    case *client.AuthorizationStateWaitPassword:
        return domain.AuthStateEvent{
            State: domain.AuthStateWaitPassword,
            Extra: &domain.WaitPasswordState{PasswordHint: s.PasswordHint},
        }
    default:
        return domain.AuthStateEvent{State: domain.AuthStateReady}
    }
}
```

| TDLib State | Domain State | Extra |
|---|---|---|
| `AuthorizationStateWaitPhoneNumber` | `AuthStateWaitPhone` | nil |
| `AuthorizationStateWaitCode` | `AuthStateWaitCode` | nil |
| `AuthorizationStateWaitPassword` | `AuthStateWaitPassword` | `&WaitPasswordState{PasswordHint}` |
| `AuthorizationStateReady` | `AuthStateReady` | nil |
| `AuthorizationStateClosing` | — | пропускается в `listenAuthStates` |
| channel closed | `AuthStateClosed` | nil |

**Источник:** `budva43/service/auth/service.go:122-159`

### T6. Static TDLib methods

#### T6.1. ParseTextEntities

Текущая заглушка возвращает пустой `FormattedText`. Заменить на:

```go
func (r *Repo) ParseTextEntities(_ context.Context, text string) (*domain.FormattedText, error) {
    result, err := client.ParseTextEntities(&client.ParseTextEntitiesRequest{
        Text: text,
        ParseMode: &client.TextParseModeMarkdown{Version: 2},
    })
    // ... map result to domain.FormattedText
}
```

**Это статический вызов** — не требует `r.client`. Работает даже до авторизации.

**Источник:** `budva43/repo/telegram/client_adapter.go:173-183`

#### T6.2. GetMarkdownText

Добавить метод в интерфейс и реализовать:

```go
func (r *Repo) GetMarkdownText(_ context.Context, text *domain.FormattedText) (*domain.FormattedText, error) {
    // client.GetMarkdownText() — static
}
```

Используется в `facade_grpc` для конвертации `FormattedText → Markdown` при отдаче через gRPC.

**Источник:** `budva43/repo/telegram/client_adapter.go:185-198`

### T7. Update listener

В budva43 обновления приходят через `client.Listener.Updates`. Нужно:

1. После `createClient()` получить listener: `r.client.GetListener()`
2. Читать `listener.Updates` в горутине
3. Конвертировать `client.Update*` → `domain.Update` и отправлять в `r.updates` канал

Маппинг:

| TDLib Update | domain.UpdateType | Поля |
|---|---|---|
| `UpdateNewMessage` | `UpdateNewMessage` | Message (с конвертацией) |
| `UpdateMessageEdited` | `UpdateMessageEdited` | Message (с ConvertMessage) |
| `UpdateDeleteMessages` | `UpdateDeleteMessages` | ChatID, MessageIDs, IsPermanent |
| `UpdateMessageSendSucceeded` | `UpdateMessageSendSucceeded` | Message, OldMessageID |

**Источник:** `budva43/service/engine/service.go:88-121`

### T8. Конвертация domain ↔ TDLib types

Создать адаптерный слой в `repo/telegram/` для маппинга:

#### T8.1. Message conversion

`client.Message` → `domain.Message`:
- ChatID, ID, Content (Text/Photo/Video/Document/Audio/Animation/VoiceNote)
- ForwardInfo (OriginChatID, OriginMessageID)
- ReplyMarkup → CallbackData

#### T8.2. InputMessageContent conversion

`domain.InputMessageContent` → `client.InputMessageContent`:
- ContentText → `InputMessageText` (с LinkPreviewOptions)
- ContentPhoto → `InputMessagePhoto` (с FilePath/FileID)
- ContentVideo → `InputMessageVideo`
- ContentAudio → `InputMessageAudio`
- ContentDocument → `InputMessageDocument`

#### T8.3. FormattedText conversion

`domain.FormattedText` ↔ `client.FormattedText`:
- Text, Entities (offset, length, type)

### T9. GetOption / GetMe — реальные вызовы

```go
func (r *Repo) GetOption(_ context.Context, name string) (string, error) {
    opt, err := client.GetOption(&client.GetOptionRequest{Name: name})  // static
    // ... extract string value from OptionValueString
}

func (r *Repo) GetMe(_ context.Context) (int64, error) {
    user, err := r.client.GetMe()  // instance
    return user.Id, err
}
```

**Источник:** `budva43/repo/telegram/client_adapter.go:210-225`

### T10. Реализация SendMessage / ForwardMessages / Edit / Delete

Заменить заглушки на реальные TDLib-вызовы:

| Метод | TDLib API |
|---|---|
| `SendMessage` | `r.client.SendMessage()` |
| `SendMessageAlbum` | `r.client.SendMessageAlbum()` |
| `ForwardMessages` | `r.client.ForwardMessages()` |
| `EditMessageText` | `r.client.EditMessageText()` |
| `EditMessageCaption` | `r.client.EditMessageCaption()` |
| `DeleteMessages` | `r.client.DeleteMessages()` |
| `GetMessage` | `r.client.GetMessage()` |
| `GetMessageLink` | `r.client.GetMessageLink()` |
| `GetMessageLinkInfo` | `r.client.GetMessageLinkInfo()` |
| `GetChatHistory` | `r.client.GetChatHistory()` |
| `TranslateText` | `r.client.TranslateText()` |
| `GetCallbackQueryAnswer` | `r.client.GetCallbackQueryAnswer()` |
| `GetChatType` | `r.client.GetChat()` → `.Type` |
| `LoadChats` | `r.client.LoadChats()` |

**Источник:** `budva43/repo/telegram/client_adapter.go` (весь файл)

### T11. Loader: LoadChats + WarmUpChat

После авторизации (Ready) вызвать:
1. `LoadChats(ctx, 200)` — загрузка списка чатов в кеш TDLib
2. `WarmUpChat(ctx, chatID, 1)` — прогрев кеша для каждого целевого чата из ruleset

В budva43 это делает `service/loader`. В budva-claude можно встроить в `cmd/engine/main.go` после `clientDone`.

**Источник:** `budva43/service/loader/service.go:131-147`

### T12. Integration test с TDLib Test DC

Перенести `budva43/test/auth_test.go`:
- `UseTestDC=true`
- Фейковые номера телефонов для тестового DC
- Полный flow: Start → Auth → Ready → GetStatus
- Использовать `termIO` mock для эмуляции ввода
- Build tag `tdlib` для пропуска без TDLib

**Источник:** `budva43/test/auth_test.go`

## Порядок выполнения

```
T1 (Dockerfile) + T2 (go-tdlib) + T3 (Config)
         ↓
T4 (repo/telegram core) + T5 (state mapping)
         ↓
T6 (static methods) + T7 (update listener) + T8 (type conversion)
         ↓
T9 (GetOption/GetMe) + T10 (CRUD operations)
         ↓
T11 (Loader) + T12 (integration test)
```

Первые три задачи (T1-T3) — инфраструктура. T4-T5 — ядро auth. T6-T8 — данные. T9-T10 — операции. T11-T12 — финализация.

## Риски

| Риск | Митигация |
|---|---|
| go-tdlib v0.7.6 может не поддерживать Go 1.25 | Проверить совместимость, при необходимости обновить |
| TDLib C++ сборка занимает ~30 мин | Использовать pre-built образ `tdlib-ubuntu` |
| `time.Sleep(1s)` при Close() — хрупкий workaround | Оставить как есть, зафиксировать TODO |
| `ParseTextEntities` / `GetMarkdownText` — static, но требуют загруженную `libtdjson.so` | Убедиться что SO доступна в runtime |
| Mapping domain ↔ TDLib types — основной объём кода | T8 — самая трудоёмкая задача, начать с неё раньше |

## Что НЕ меняется

- Вся бизнес-логика (handler, services) — без изменений
- domain types — без изменений
- transport layer (grpc, http, term) — без изменений
- test/support/FakeTelegram — остаётся для unit/BDD/integration тестов
- auth.Service — без изменений (AuthStates/InputChan/Subscribe/Close уже готовы)
- Интерфейс `clientAdapter` — без изменений (меняется только реализация методов)
