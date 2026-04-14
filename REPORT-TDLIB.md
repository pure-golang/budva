# Phase B: план интеграции TDLib

Последняя актуализация: 2026-04-14.

## Предусловия

Phase A полностью закрыта. Вся бизнес-логика (handler, transform, filters, dedup, album, limiter, message, facade, auth) работает через абстракции `domain.*` и интерфейсы. TDLib-зависимый код локализован в `internal/repo/telegram/`.

**Текущее состояние `repo/telegram`:** заглушка с `RunAuthFlow` (WaitPhone → WaitCode → WaitPassword → Ready), остальные методы возвращают nil/пустые значения.

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

Заменить заглушку на реальную TDLib-интеграцию.

#### T4.1. Структура Repo

```go
type Repo struct {
    logger     *slog.Logger
    cfg        config.TelegramConfig
    client     *client.Client       // go-tdlib клиент
    clientDone chan struct{}
    updates    chan domain.Update
}
```

#### T4.2. Start() + setupClientLog()

- Вызвать `client.SetLogStream()` → файл `cfg.LogDirectory/telegram.log`
- Вызвать `client.SetLogVerbosityLevel()` → `cfg.LogVerbosityLevel`

**Источник:** `budva43/repo/telegram/repo.go:150-169`

#### T4.3. CreateClient() с retry loop

```go
func (r *Repo) CreateClient(handler func() client.AuthorizationStateHandler, onReady func()) {
    for {
        authorizer := handler()
        tdlibClient, err := client.NewClient(authorizer)
        if err != nil {
            r.logger.Error("Failed to create TDLib client", slog.Any("err", err))
            continue
        }
        r.client = tdlibClient
        onReady()
        close(r.clientDone)
        return
    }
}
```

**Источник:** `budva43/repo/telegram/repo.go:82-113`

#### T4.4. Close() с sleep workaround

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

### T5. Auth flow → реальный TDLib

Заменить `RunAuthFlow` на реальный `newFuncRunAuthorizationStateHandler`:

```go
func (r *Repo) newAuthHandler(ctx context.Context, auth authDriver) client.AuthorizationStateHandler {
    authorizer := client.ClientAuthorizer(r.CreateTdlibParameters())
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case state, ok := <-authorizer.State:
                if !ok {
                    auth.SetState(domain.AuthStateClosed, nil)
                    return
                }
                if _, isClosing := state.(*client.AuthorizationStateClosing); isClosing {
                    continue // пропускаем broadcast
                }
                domainState, extra := mapTDLibState(state)
                auth.SetState(domainState, extra)
                switch state.(type) {
                case *client.AuthorizationStateWaitPhoneNumber:
                    authorizer.PhoneNumber <- <-auth.ReadChan()
                case *client.AuthorizationStateWaitCode:
                    authorizer.Code <- <-auth.ReadChan()
                case *client.AuthorizationStateWaitPassword:
                    authorizer.Password <- <-auth.ReadChan()
                }
            }
        }
    }()
    return authorizer
}
```

Маппинг TDLib states → domain states:

| TDLib State | Domain State | Extra |
|---|---|---|
| `AuthorizationStateWaitPhoneNumber` | `AuthStateWaitPhone` | nil |
| `AuthorizationStateWaitCode` | `AuthStateWaitCode` | nil |
| `AuthorizationStateWaitPassword` | `AuthStateWaitPassword` | `&WaitPasswordState{PasswordHint}` |
| `AuthorizationStateReady` | `AuthStateReady` | nil |
| `AuthorizationStateClosing` | — | пропускается |
| `AuthorizationStateClosed` | `AuthStateClosed` | nil |

**Источник:** `budva43/service/auth/service.go:122-159`

### T6. Static TDLib methods

#### T6.1. ParseTextEntities

Текущая заглушка в `repo.go` возвращает пустой `FormattedText`. Заменить на:

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

1. После `CreateClient()` получить listener: `r.client.GetListener()`
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
- TermAutomator для эмуляции stdin (или использовать `termIO` mock)
- Build tag `tdlib` для пропуска без TDLib

**Источник:** `budva43/test/auth_test.go`

## Порядок выполнения

```
T1 (Dockerfile) + T2 (go-tdlib) + T3 (Config)
         ↓
T4 (repo/telegram core)
         ↓
T5 (auth flow) + T6 (static methods)
         ↓
T7 (update listener) + T8 (type conversion)
         ↓
T9 (GetOption/GetMe) + T10 (CRUD operations)
         ↓
T11 (Loader) + T12 (integration test)
```

Первые три задачи (T1-T3) — инфраструктура. T4-T6 — ядро. T7-T10 — всё остальное. T11-T12 — финализация.

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
- auth.Service — без изменений (SetState/ReadChan уже готовы)
