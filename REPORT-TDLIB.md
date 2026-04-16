# Phase B: план интеграции TDLib

Последняя актуализация: 2026-04-16 (v4).

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

## Принятые решения

| Вопрос | Решение | Обоснование |
|---|---|---|
| Версия go-tdlib | **v0.7.6** | Консистентность с budva43 |
| Коммит TDLib | **`22d49d5`** | Из Makefile тега v0.7.6 |
| Test DC | **Не используем** | Сломан с середины 2025 (tdlib/td#3370, #3564) |
| Telegram-аккаунт | **Реальный, через `.env`** | Как в budva43 |
| DevContainer | **Нет** | Сборка TDLib локально один раз (~30 мин) |
| Docker base | **Bookworm** (Debian) | Остаёмся на glibc |
| Разработка | **Нативно на macOS** | TDLib в `/usr/local`, `go build` без Docker |
| Build tags | **Нет** | Заглушки заменяются in-place |
| FakeTelegram | **Удаляется** после Phase B | Строительные леса Phase A |
| Unit-тесты | **Mockery** | Частично применяемые интерфейсы |
| BDD-тесты | **Живой TDLib** | Реальные чаты из `cmd/stand --up` |

## Зависимости

```
github.com/zelenin/go-tdlib v0.7.6
```

TDLib коммит: `22d49d5b87a4d5fc60a194dab02dd1d71529687f` (из Makefile go-tdlib v0.7.6).

Совместимость с Go 1.25: go-tdlib v0.7.6 не содержит директивы `go` в go.mod — совместим с любой версией.

## Локальная сборка TDLib (macOS Intel)

Инструкции сгенерированы на https://tdlib.github.io/td/build.html (Go + macOS + Intel + Install to /usr/local).

```bash
$ xcode-select --install
$ /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
$ brew install gperf cmake openssl
$ git clone https://github.com/tdlib/td.git
$ cd td
$ git checkout 22d49d5
$ rm -rf build
$ mkdir build
$ cd build
$ cmake -DCMAKE_BUILD_TYPE=Release -DOPENSSL_ROOT_DIR=/usr/local/opt/openssl/ -DCMAKE_INSTALL_PREFIX:PATH=/usr/local ..
$ cmake --build . --target install
```

После установки `go build ./...` работает нативно — TDLib headers и libs в `/usr/local`.

Для других платформ: https://tdlib.github.io/td/build.html — выбрать язык Go, ОС, архитектуру, галку «Install to /usr/local».

Если TDLib установлен в нестандартный путь:

```bash
CGO_CFLAGS=-I/path/to/tdlib/include \
CGO_LDFLAGS="-Wl,-rpath,/path/to/tdlib/lib -L/path/to/tdlib/lib -ltdjson" \
go build ...
```

## Задачи

### T1. Dockerfile с TDLib (Bookworm)

Текущий `Dockerfile` собирает без CGO. Нужно добавить stage сборки TDLib.

**Целевой Dockerfile:**

```dockerfile
# Stage 0: Сборка TDLib C++
FROM dockerhub.timeweb.cloud/library/debian:bookworm AS tdlib-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake g++ gperf libssl-dev zlib1g-dev php-cli git make ca-certificates

RUN git clone https://github.com/tdlib/td.git /td && \
    cd /td && git checkout 22d49d5

RUN cd /td && mkdir build && cd build && \
    cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr/local .. && \
    cmake --build . --target prepare_cross_compiling && \
    cd .. && php SplitSource.php && cd build && \
    cmake --build . --target install

# Stage 1: Go builder
FROM dockerhub.timeweb.cloud/library/golang:1.25.9-bookworm AS builder

COPY --from=tdlib-builder /usr/local/include/td /usr/local/include/td/
COPY --from=tdlib-builder /usr/local/lib/libtd* /usr/local/lib/
RUN ldconfig

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=1 go build -a -trimpath -ldflags "-s -w" -o /bin/facade ./cmd/facade
RUN CGO_ENABLED=1 go build -a -trimpath -ldflags "-s -w" -o /bin/engine ./cmd/engine
RUN CGO_ENABLED=1 go build -a -trimpath -ldflags "-s -w" -o /bin/stand ./cmd/stand

# Stage 2: Runtime
FROM dockerhub.timeweb.cloud/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libstdc++6 libssl3 zlib1g && \
    rm -rf /var/lib/apt/lists/*

COPY --from=tdlib-builder /usr/local/lib/libtd* /usr/local/lib/
RUN ldconfig

RUN adduser --disabled-password --gecos '' appuser
USER appuser

COPY --from=builder /bin/facade /facade
COPY --from=builder /bin/engine /engine
COPY --from=builder /bin/stand /stand
COPY --from=builder /app/ruleset.yml /ruleset.yml
COPY --from=builder /app/.env.example /.env

EXPOSE 7070
ENTRYPOINT ["/facade"]
```

Stage `tdlib-builder` кешируется Docker layer cache — пересобирается только при смене коммита TDLib (~30 мин первый раз, потом мгновенно).

### T2. go-tdlib зависимость

```bash
go get github.com/zelenin/go-tdlib@v0.7.6
```

Добавить в `go.mod`. Заглушки в `client_adapter.go` заменяются на реальные вызовы go-tdlib in-place.

### T3. Config: параметры TDLib + реальный аккаунт

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

Убрать `UseTestDC` из конфига (Test DC не работает).

Обновить `.env.example`:

```env
TELEGRAM_API_ID=
TELEGRAM_API_HASH=
TELEGRAM_PHONE=
```

Реальный аккаунт прописывается в `.env` (в `.gitignore`). Телефон отправляется автоматически при авторизации, код и пароль — через терминал. Аналогично budva43.

**Источник:** `budva43/.config/.private/.env`, `budva43/repo/telegram/repo.go:63-80`

### T4. repo/telegram — TDLib клиент

Заменить fake-реализацию на реальную TDLib-интеграцию. Все изменения внутри `Repo` — интерфейс `clientAdapter` не меняется.

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

```go
func (r *Repo) runAuthLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        authorizer := client.ClientAuthorizer(r.createTdlibParameters())

        r.mu.Lock()
        r.authorizer = authorizer
        r.mu.Unlock()

        go r.listenAuthStates(ctx, authorizer.State)

        tdlibClient, err := client.NewClient(authorizer)
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

Authorizer создаётся заново на каждую попытку. При failure старый authorizer.State закрывается → `listenAuthStates` завершается → `auth.Service.run()` фильтрует AuthStateClosed (continue) → новая итерация эмитит свежий WaitPhone.

#### T4.4. listenAuthStates() — маппинг TDLib → domain

```go
func (r *Repo) listenAuthStates(ctx context.Context, states <-chan client.AuthorizationState) {
    for {
        select {
        case <-ctx.Done():
            return
        case state, ok := <-states:
            if !ok {
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

#### T4.5. Submit* — делегирование в authorizer (с синхронизацией)

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

#### T4.6. Close()

```go
func (r *Repo) Close() error {
    if r.client == nil {
        return nil
    }
    _, err := r.client.Close()
    r.client = nil
    return err
}
```

Примечание: внутри `client.NewClient()` go-tdlib делает `time.Sleep(1 * time.Second)` после `AuthorizationStateReady` («dirty hack for db flush after authorization» — `authorization.go:49`). Это внутри библиотеки, не в нашем коде.

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

```go
func (r *Repo) ParseTextEntities(_ context.Context, text string) (*domain.FormattedText, error) {
    result, err := client.ParseTextEntities(&client.ParseTextEntitiesRequest{
        Text: text,
        ParseMode: &client.TextParseModeMarkdown{Version: 2},
    })
    // ... map result to domain.FormattedText
}
```

Статический вызов — не требует `r.client`. Работает до авторизации.

**Источник:** `budva43/repo/telegram/client_adapter.go:173-183`

#### T6.2. GetMarkdownText

```go
func (r *Repo) GetMarkdownText(_ context.Context, text *domain.FormattedText) (*domain.FormattedText, error) {
    // client.GetMarkdownText() — static
}
```

**Источник:** `budva43/repo/telegram/client_adapter.go:185-198`

### T7. Update listener

После `client.NewClient()` в `runAuthLoop()`:

1. Получить listener: `r.client.GetListener()`
2. Читать `listener.Updates` в горутине
3. Конвертировать `client.Update*` → `domain.Update` → `r.updates`

| TDLib Update | domain.UpdateType | Поля |
|---|---|---|
| `UpdateNewMessage` | `UpdateNewMessage` | Message |
| `UpdateMessageEdited` | `UpdateMessageEdited` | Message |
| `UpdateDeleteMessages` | `UpdateDeleteMessages` | ChatID, MessageIDs, IsPermanent |
| `UpdateMessageSendSucceeded` | `UpdateMessageSendSucceeded` | Message, OldMessageID |

**Источник:** `budva43/service/engine/service.go:88-121`

### T8. Тонкий маппинг domain ↔ go-tdlib types

go-tdlib — источник правды. Маппинг механический, inline в каждом методе `repo/telegram/`.

- `client.Message` → `domain.Message` (ChatID, ID, Content, ForwardInfo, ReplyTo)
- `domain.InputMessageContent` → `client.InputMessageContent` (Text, Photo, Video и т.д.)
- `client.FormattedText` ↔ `domain.FormattedText` (Text, Entities)

### T9. GetOption / GetMe

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
| `GetChatType` | `r.client.GetChat()` → `.Type` | `"supergroup"` или `"basicGroup"` |
| `LoadChats` | `r.client.LoadChats()` | |
| `CreateNewSupergroupChat` | `r.client.CreateNewSupergroupChat()` | `(chatID, supergroupID, error)` |
| `CreateNewBasicGroupChat` | `r.client.CreateNewBasicGroupChat()` | |
| `SetSupergroupUsername` | `r.client.SetSupergroupUsername()` | принимает supergroupID |
| `DeleteChat` | `r.client.DeleteChat()` | |

**Источник:** `budva43/repo/telegram/client_adapter.go`

### T11. Loader: LoadChats + WarmUpChat

После авторизации (Ready):
1. `LoadChats(ctx, 200)` — загрузка списка чатов в кеш TDLib
2. `WarmUpChat(ctx, chatID, 1)` — прогрев кеша для каждого целевого чата из ruleset

**Источник:** `budva43/service/loader/service.go:131-147`

### T12. Удаление FakeTelegram, перевод тестов на реальный TDLib

`FakeTelegram` — строительные леса Phase A. После подключения TDLib удаляется.

**Удалить:**
- `test/support/fake_telegram.go`
- `test/support/stack.go`

**Переписать:**
- `test/bdd/steps/context_test.go` — заменить `Stack` на реальный `telegram.Repo` + фикстуры из `.config/stand.json`
- `test/bdd/steps/*_steps_test.go` — заменить `FakeTelegram.PutMessage()` / `MessagesInChat()` на реальные TDLib-вызовы

**Тестовая стратегия после Phase B:**

| Слой | Что мокается | Что реальное |
|---|---|---|
| unit | зависимости через mockery (частично применяемые интерфейсы) | бизнес-логика |
| bdd | ничего | всё: TDLib + services + handler, чаты из `cmd/stand --up` |

**BDD prerequisites:**
1. TDLib собран локально (или Docker)
2. Реальный Telegram-аккаунт в `.env`
3. Тестовые чаты развёрнуты (`cmd/stand --up`)
4. Фикстуры загружены из `.config/stand.json`

## Порядок выполнения

```
T1 (Dockerfile) + T2 (go-tdlib) + T3 (Config + .env)
         ↓  checkpoint: docker build + go build (локально с TDLib)
T4 (repo/telegram core) + T5 (state mapping)
         ↓  checkpoint: auth flow через реальный TDLib
T6 (static methods) + T7 (update listener)
         ↓
T8 (маппинг inline) + T9 (GetOption/GetMe) + T10 (все методы clientAdapter)
         ↓  checkpoint: cmd/stand --up создаёт реальные чаты
T11 (Loader)
         ↓
T12 (удаление FakeTelegram, BDD через реальный TDLib)
```

- T1-T3 — инфраструктура: Docker, зависимость, конфиг, реальный аккаунт.
- T4-T5 — ядро auth.
- T6-T7 — static methods и update events.
- T8-T10 — реализация всех методов (маппинг inline).
- T11 — загрузка чатов.
- T12 — удаление строительных лесов, перевод BDD на живой TDLib.

## Риски

| Риск | Митигация |
|---|---|
| Сборка TDLib ~30 мин | Один раз на рабочей машине; Docker кеширует stage |
| BDD зависят от Telegram API | Rate limits, сетевые ошибки; выделенный аккаунт, retry в шагах |
| Реальный аккаунт может быть заблокирован | Не автоматизировать агрессивно; использовать отдельную SIM |

## Что НЕ меняется

- Вся бизнес-логика (handler, services) — без изменений
- domain types — без изменений
- transport layer (grpc, http, term) — без изменений
- test/support/fixtures.go — маппинг BDD Examples → реальные chat ID из `.config/stand.json`
- auth.Service — без изменений (AuthStates/InputChan/Subscribe/Close уже готовы)
- Интерфейс `clientAdapter` — определён полностью, включая stand-методы. При подключении TDLib меняется только реализация методов
- `cmd/stand/` — утилита для управления тестовыми чатами (up/down), фикстуры в `.config/stand.json`
