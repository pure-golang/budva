# Phase B: план интеграции TDLib

Последняя актуализация: 2026-04-16 (v3).

## Предусловия

Phase A полностью закрыта. Вся бизнес-логика (handler, transform, filters, dedup, album, limiter, message, facade, auth) работает через абстракции `domain.*` и интерфейсы. TDLib-зависимый код локализован в `internal/repo/telegram/`.

**Текущее состояние `repo/telegram`:** fake-реализация с event-driven auth flow (SubmitPhone/Code/Password эмитят события в `authStates` канал), опциональный WaitPassword через `has2FA`. Интерфейс `clientAdapter` полностью определён (включая `forAlbum` в `GetMessageLink`, batch `GetMessages`, `GetMarkdownText`, stand-методы для управления чатами), методы возвращают nil/пустые значения.

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

TDLib коммит, совместимый с go-tdlib v0.7.6: `22d49d5b87a4d5fc60a194dab02dd1d71529687f` (short: `22d49d5`).

## Docker-образы

| Образ | Назначение | Проверен |
|---|---|---|
| `ghcr.io/zelenin/tdlib-docker:22d49d5-alpine` | Pre-built TDLib C++ (headers + libs) для go-tdlib v0.7.6 | ✅ `docker manifest inspect` OK |
| `dockerhub.timeweb.cloud/library/golang:1.25.9-alpine` | Go builder (musl, CGO) | ✅ `docker manifest inspect` OK |
| `dockerhub.timeweb.cloud/library/alpine:3.21` | Runtime | ✅ |

Все образы Alpine (musl libc). Сборка и runtime на одной платформе — без конфликтов glibc/musl. Docker-контейнер запускается на любом хосте (macOS, Linux).

## Задачи

### T1. Dockerfile → Alpine + TDLib

Текущий `Dockerfile` использует `golang:1.25.9-bookworm` + `debian:bookworm-slim` и собирает без CGO. Нужно переписать на Alpine с TDLib.

**Текущий Dockerfile:**

```dockerfile
FROM dockerhub.timeweb.cloud/library/golang:1.25.9-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/facade ./cmd/facade
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/engine ./cmd/engine

FROM dockerhub.timeweb.cloud/library/debian:bookworm-slim
RUN adduser --disabled-password --gecos '' appuser
USER appuser
COPY --from=builder /bin/facade /facade
COPY --from=builder /bin/engine /engine
COPY --from=builder /app/ruleset.yml /ruleset.yml
COPY --from=builder /app/.env.example /.env
EXPOSE 7070
ENTRYPOINT ["/facade"]
```

**Целевой Dockerfile:**

```dockerfile
# Stage 0: Pre-built TDLib для go-tdlib v0.7.6
FROM ghcr.io/zelenin/tdlib-docker:22d49d5-alpine AS tdlib

# Stage 1: Go builder с TDLib
FROM dockerhub.timeweb.cloud/library/golang:1.25.9-alpine AS builder

RUN apk add --no-cache \
    bash \
    build-base \
    ca-certificates \
    git \
    linux-headers \
    openssl-dev \
    zlib-dev

# TDLib headers и библиотеки
COPY --from=tdlib /usr/local/include/td /usr/local/include/td/
COPY --from=tdlib /usr/local/lib/libtd* /usr/local/lib/

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# CGO_ENABLED=1 — TDLib линкуется через cgo
RUN CGO_ENABLED=1 go build -a -trimpath -ldflags "-s -w" -o /bin/facade ./cmd/facade
RUN CGO_ENABLED=1 go build -a -trimpath -ldflags "-s -w" -o /bin/engine ./cmd/engine
RUN CGO_ENABLED=1 go build -a -trimpath -ldflags "-s -w" -o /bin/stand ./cmd/stand

# Stage 2: Runtime
FROM dockerhub.timeweb.cloud/library/alpine:3.21

RUN apk add --no-cache ca-certificates libstdc++
RUN adduser -D appuser
USER appuser

COPY --from=builder /bin/facade /facade
COPY --from=builder /bin/engine /engine
COPY --from=builder /bin/stand /stand
COPY --from=builder /app/ruleset.yml /ruleset.yml
COPY --from=builder /app/.env.example /.env

EXPOSE 7070
ENTRYPOINT ["/facade"]
```

**Ключевые отличия от текущего:**

| Аспект | Было (Bookworm) | Стало (Alpine) |
|---|---|---|
| Base image | `golang:1.25.9-bookworm` | `golang:1.25.9-alpine` |
| Runtime | `debian:bookworm-slim` | `alpine:3.21` |
| CGO | `CGO_ENABLED=0` | `CGO_ENABLED=1` |
| TDLib | нет | `ghcr.io/zelenin/tdlib-docker:22d49d5-alpine` |
| libc | glibc | musl |
| Runtime deps | нет | `ca-certificates libstdc++` |
| Build deps | нет | `build-base openssl-dev zlib-dev linux-headers` |
| `adduser` | `--disabled-password --gecos ''` | `-D` (Alpine syntax) |
| `GOOS/GOARCH` | явно заданы | не нужны (нативная сборка внутри контейнера) |
| stand binary | нет | собирается и копируется |

**Запуск на macOS/Linux:** Docker Desktop для macOS запускает контейнер в Linux VM — Alpine работает одинаково на обоих хостах.

**Источник:** `zelenin/go-tdlib/example/Dockerfile`

### T2. go-tdlib зависимость

```bash
go get github.com/zelenin/go-tdlib@v0.7.6
```

Добавить в `go.mod`. Сборка и тестирование — только внутри Docker (TDLib headers/libs нужны для CGO). Локальная сборка на хосте не предусмотрена.

Заглушки в `client_adapter.go` заменяются на реальные вызовы go-tdlib in-place. Build tags не нужны — разделения stub/real нет.

### T3. TDLib parameters в Config

Добавить в `TelegramConfig` недостающие поля:

| Поле | Тип | Envconfig | Default | Описание |
|---|---|---|---|---|
| `UseFileDatabase` | bool | `TELEGRAM_USE_FILE_DB` | true | Файловый кеш TDLib |
| `UseChatInfoDatabase` | bool | `TELEGRAM_USE_CHAT_INFO_DB` | true | Кеш информации о чатах |
| `UseMessageDatabase` | bool | `TELEGRAM_USE_MESSAGE_DB` | true | Кеш сообщений |
| `UseSecretChats` | bool | `TELEGRAM_USE_SECRET_CHATS` | false | Поддержка секретных чатов |
| `SystemVersion` | string | `TELEGRAM_SYSTEM_VERSION` | "" | Версия системы |
| `ApplicationVersion` | string | `TELEGRAM_APP_VERSION` | "" | Версия приложения |
| `LogDirectory` | string | `TELEGRAM_LOG_DIR` | ".data/tdlib-logs" | Директория логов TDLib |
| `LogMaxFileSize` | int64 | `TELEGRAM_LOG_MAX_SIZE` | 10 | Макс размер лог-файла (MB) |

Обновить `.env.example` с новыми переменными (закомментированными).

**Источник:** `budva43/repo/telegram/repo.go:63-80`

### T4. repo/telegram — TDLib клиент

Заменить fake-реализацию на реальную TDLib-интеграцию. Все изменения внутри `Repo` — интерфейс `clientAdapter` не меняется.

**Build tags:** реализация разделяется на два файла:
- `client_adapter_stub.go` (`//go:build !tdlib`) — текущие заглушки
- `client_adapter_tdlib.go` (`//go:build tdlib`) — реальные вызовы

#### T4.1. Структура Repo

```go
type Repo struct {
    logger     *slog.Logger
    cfg        config.TelegramConfig
    client     *client.Client              // go-tdlib клиент (nil до авторизации)
    mu         sync.RWMutex                // защита authorizer при retry
    authorizer *client.ClientAuthorizer    // каналы для phone/code/password
    clientDone chan struct{}
    updates    chan domain.Update
    authStates chan domain.AuthStateEvent
}
```

Поле `has2FA` удаляется — TDLib сам решает, нужен ли WaitPassword.

#### T4.2. Start() + setupClientLog()

```go
func (r *Repo) Start(ctx context.Context) error {
    if err := r.setupClientLog(); err != nil {
        return err
    }
    go r.runAuthLoop(ctx)
    return nil
}
```

`setupClientLog()`:
- `client.SetLogStream()` → файл `cfg.LogDirectory/telegram.log`
- `client.SetLogVerbosityLevel()` → `cfg.LogVerbosityLevel`

**Источник:** `budva43/repo/telegram/repo.go:150-169`

#### T4.3. runAuthLoop() — единый цикл авторизации с retry

В legacy каждая итерация retry создаёт свежий authorizer и свежую горутину-listener (`budva43/repo/telegram/repo.go:82-113`). Наша архитектура повторяет эту семантику:

```go
func (r *Repo) runAuthLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        // Свежий authorizer на каждую попытку (как в legacy)
        authorizer := client.ClientAuthorizer(r.createTdlibParameters())

        r.mu.Lock()
        r.authorizer = authorizer
        r.mu.Unlock()

        // Горутина: слушает authorizer.State, маппит в domain events.
        // Завершится когда authorizer.State закроется (при failure или success).
        go r.listenAuthStates(ctx, authorizer.State)

        // client.NewClient блокируется до завершения авторизации.
        // При неверном коде/пароле TDLib переэмитит состояние →
        // listenAuthStates отправит его в authStates →
        // auth.Service.run() уведомит транспорт → пользователь повторит ввод.
        tdlibClient, err := client.NewClient(authorizer)
        if err != nil {
            r.logger.Error("Failed to create TDLib client", slog.Any("err", err))
            continue // новая попытка с новым authorizer
        }

        r.client = tdlibClient
        close(r.clientDone)
        return
    }
}
```

**Ключевое отличие от старого плана:** authorizer создаётся заново на каждую попытку, а не переиспользуется. При failure старый authorizer.State закрывается → `listenAuthStates` завершается → `auth.Service.run()` фильтрует AuthStateClosed (continue) → новая итерация эмитит свежий WaitPhone.

#### T4.4. listenAuthStates() — маппинг TDLib → domain

Принимает конкретный канал `authorizer.State`, а не поле `r.authorizer`. Это позволяет нескольким горутинам при retry не конфликтовать:

```go
func (r *Repo) listenAuthStates(ctx context.Context, states <-chan client.AuthorizationState) {
    for {
        select {
        case <-ctx.Done():
            return
        case state, ok := <-states:
            if !ok {
                return // authorizer закрыт (failure или shutdown)
            }
            if _, isClosing := state.(*client.AuthorizationStateClosing); isClosing {
                continue
            }
            r.authStates <- mapTDLibState(state)
        }
    }
}
```

#### T4.5. Submit* — делегирование в authorizer (с синхронизацией)

`r.authorizer` может быть заменён при retry, поэтому чтение под RLock:

```go
func (r *Repo) SubmitPhone(_ context.Context, phone string) error {
    r.logger.Info("Phone submitted", slog.String("phone", domain.MaskPhoneNumber(phone)))
    r.mu.RLock()
    authorizer := r.authorizer
    r.mu.RUnlock()
    authorizer.PhoneNumber <- phone
    return nil
}

func (r *Repo) SubmitCode(_ context.Context, code string) error {
    r.logger.Info("Code submitted")
    r.mu.RLock()
    authorizer := r.authorizer
    r.mu.RUnlock()
    authorizer.Code <- code
    return nil
}

func (r *Repo) SubmitPassword(_ context.Context, password string) error {
    r.logger.Info("Password submitted")
    r.mu.RLock()
    authorizer := r.authorizer
    r.mu.RUnlock()
    authorizer.Password <- password
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

### T8. Тонкий маппинг domain ↔ go-tdlib types

go-tdlib — источник правды. Domain types уже спроектированы по образу TDLib, поэтому маппинг — механический, без отдельного «адаптерного слоя». Конвертация живёт в `repo/telegram/` рядом с методами.

Основные точки маппинга:
- `client.Message` → `domain.Message` (ChatID, ID, Content, ForwardInfo, ReplyTo)
- `domain.InputMessageContent` → `client.InputMessageContent` (Text, Photo, Video и т.д.)
- `client.FormattedText` ↔ `domain.FormattedText` (Text, Entities)

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

### T10. Реализация остальных методов clientAdapter

Заменить заглушки на реальные TDLib-вызовы:

| Метод | TDLib API | Примечание |
|---|---|---|
| `SendMessage` | `r.client.SendMessage()` | |
| `SendMessageAlbum` | `r.client.SendMessageAlbum()` | |
| `ForwardMessages` | `r.client.ForwardMessages()` | |
| `EditMessageText` | `r.client.EditMessageText()` | |
| `EditMessageCaption` | `r.client.EditMessageCaption()` | |
| `DeleteMessages` | `r.client.DeleteMessages()` | |
| `GetMessage` | `r.client.GetMessage()` | |
| `GetMessages` | `r.client.GetMessages()` | batch по списку ID |
| `GetMessageLink` | `r.client.GetMessageLink()` | `forAlbum` уже в сигнатуре |
| `GetMessageLinkInfo` | `r.client.GetMessageLinkInfo()` | |
| `GetChatHistory` | `r.client.GetChatHistory()` | |
| `TranslateText` | `r.client.TranslateText()` | |
| `GetCallbackQueryAnswer` | `r.client.GetCallbackQueryAnswer()` | |
| `GetChatType` | `r.client.GetChat()` → `.Type` | возвращает `"supergroup"` или `"basicGroup"` |
| `LoadChats` | `r.client.LoadChats()` | bypass `getClient()` |
| `CreateNewSupergroupChat` | `r.client.CreateNewSupergroupChat()` | возвращает `(chatID, supergroupID, error)` |
| `CreateNewBasicGroupChat` | `r.client.CreateNewBasicGroupChat()` | |
| `SetSupergroupUsername` | `r.client.SetSupergroupUsername()` | принимает supergroupID, не chatID |
| `DeleteChat` | `r.client.DeleteChat()` | |

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

### T13. Удаление FakeTelegram, перевод тестов на реальный TDLib

`FakeTelegram` — строительные леса Phase A. После подключения TDLib удаляется.

**Удалить:**
- `test/support/fake_telegram.go`
- `test/support/stack.go` (зависит от FakeTelegram)

**Переписать:**
- `test/bdd/steps/context_test.go` — `scenarioCtx` использует `Stack` с FakeTelegram. Заменить на реальный `telegram.Repo` + фикстуры из `.config/stand.json`
- `test/bdd/steps/*_steps_test.go` — шаги Given/Then работают через `FakeTelegram.PutMessage()` / `MessagesInChat()`. Заменить на реальные TDLib-вызовы через `Repo`

**Тестовая стратегия после Phase B:**

| Слой | Что мокается | Что реальное |
|---|---|---|
| unit | зависимости через mockery (частично применяемые интерфейсы) | бизнес-логика |
| bdd | ничего | всё: TDLib + services + handler, чаты из `cmd/stand --up` |

**BDD prerequisites:**
1. Docker-контейнер с TDLib
2. Авторизованная сессия (`cmd/engine` или `cmd/stand`)
3. Тестовые чаты развёрнуты (`cmd/stand --up`)
4. Фикстуры загружены из `.config/stand.json`

**Порядок:** T13 выполняется после T10 (когда все методы clientAdapter реализованы).

## Порядок выполнения

```
T1 (Dockerfile → Alpine) + T2 (go-tdlib) + T3 (Config)
         ↓  checkpoint: docker build
T4 (repo/telegram core) + T5 (state mapping)
         ↓  checkpoint: auth flow через реальный TDLib
T6 (static methods) + T7 (update listener)
         ↓
T8 (маппинг inline) + T9 (GetOption/GetMe) + T10 (все методы clientAdapter)
         ↓  checkpoint: cmd/stand --up создаёт реальные чаты
T11 (Loader) + T12 (integration test)
         ↓
T13 (удаление FakeTelegram, BDD через реальный TDLib)
```

- T1-T3 — инфраструктура Docker + config.
- T4-T5 — ядро auth.
- T6-T7 — данные и events.
- T8-T10 — реализация всех методов (маппинг типов — inline, не отдельный слой).
- T11-T12 — загрузка чатов и integration test.
- T13 — удаление строительных лесов, перевод BDD на живой TDLib.

## Риски

| Риск | Митигация |
|---|---|
| go-tdlib v0.7.6 может не поддерживать Go 1.25 | Проверить совместимость, при необходимости обновить go-tdlib |
| `ghcr.io/zelenin/tdlib-docker:22d49d5-alpine` — single-platform (amd64) | Для arm64 (Apple Silicon) Docker Desktop использует qemu-эмуляцию; если медленно — собрать TDLib для arm64 отдельным stage |
| `time.Sleep(1s)` при Close() — хрупкий workaround | Оставить как есть, зафиксировать TODO |
| `ParseTextEntities` / `GetMarkdownText` — static, но требуют загруженную `libtdjson.so` | Работают только внутри Docker-контейнера (где SO доступна) |
| Mapping domain ↔ go-tdlib types | Маппинг механический, go-tdlib — источник правды |
| Локальная разработка | Сборка и тесты — только внутри Docker; IDE подсветка может ломаться без TDLib headers |
| BDD через живой TDLib | Тесты зависят от Telegram API (rate limits, сетевые ошибки); использовать Test DC где возможно |

## Что НЕ меняется

- Вся бизнес-логика (handler, services) — без изменений
- domain types — без изменений
- transport layer (grpc, http, term) — без изменений
- test/support/fixtures.go — маппинг BDD Examples → реальные chat ID из `.config/stand.json`
- auth.Service — без изменений (AuthStates/InputChan/Subscribe/Close уже готовы)
- Интерфейс `clientAdapter` — определён полностью, включая stand-методы (CreateNewSupergroupChat, CreateNewBasicGroupChat, SetSupergroupUsername, DeleteChat). При подключении TDLib меняется только реализация методов
- `cmd/stand/` — утилита для управления тестовыми чатами (up/down), фикстуры в `.config/stand.json`
