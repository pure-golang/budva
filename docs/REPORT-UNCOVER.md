# Отчёт: пакеты с покрытием ниже 80%

Дата: 2026-04-14. Источник: `task cover`, общее покрытие **67.8%**.

---

## Сводка

| Пакет | Покрытие | Непокрытые строки — основная причина |
|-------|----------|--------------------------------------|
| internal/controller | 65.0% | `/ready` endpoint не тестируется |
| internal/domain | 60.5% | тесты только для `phone.go` |
| internal/handler | 72.4% | `OnEditedMessage`, time-dependent goroutines |
| internal/repo/ruleset | 70.4% | `WatchContext` (fsnotify), ветки валидации |
| internal/repo/telegram | 42.0% | Phase-A заглушки (17+ stub-методов) |
| internal/repo/term | 0.0% | terminal ioctl (`ReadPassword`) |
| internal/service/facade | 0.0% | тонкий proxy без логики |
| internal/service/message | 61.5% | 4 content-type ветки `BuildInputContent` |
| internal/service/transform | 46.7% | `replaceMyselfLinks` + `findCopyLink` chain |
| internal/transport/term | 51.8% | `Run`, `processAuth` phone/code paths |

---

## Детальный разбор

### internal/controller — 65.0%

**Что не покрыто:** endpoint `/ready` — структурный клон `healthcheck`, но без собственного теста.

**Причина:** при написании тестов покрыли `/healthcheck` (happy + unhappy), а `/ready` пропустили как идентичный по логике.

**Рекомендация:** добавить `TestReady_all_healthy` и `TestReady_unhealthy` — copy-paste из существующих тестов healthcheck с заменой пути. Ожидаемый прирост: ~95%.

---

### internal/domain — 60.5%

**Что не покрыто:**
- `AuthorizationState.String()` — 7 веток включая `"unknown"`
- `FormattedText.DeepCopy()` — nil receiver, non-nil с entities
- `ContentTypeByFileExt()` — все ветки расширений
- `MessageContentType.IsMediaType()`, `HasCaption()`

**Причина:** тест-файл существует только для `phone.go`. Остальные helpers считались «очевидными».

**Рекомендация:** добавить `domain_test.go` с табличными тестами для `String()`, `DeepCopy()`, `ContentTypeByFileExt()`, `IsMediaType()`. Всё — чистые функции без зависимостей. Ожидаемый прирост: ~95%.

---

### internal/handler — 72.4%

**Что не покрыто:**
- `OnEditedMessage` целиком (edit sync, `CopyOnce` versioning, caption vs text edit, reply-markup sync)
- `processMessage` — ветка `rule.Other != 0` (FiltersOther)
- `getOriginMessage` — разворачивание forwarded-from-channel
- `resolveReplyTo` — reply-chain через `GetCopiedMessageIDs`/`GetNewMessageID`
- `runNextLinkWorkflow` — goroutine с retry-loop (10 × 1s sleep)
- `processMediaAlbum` — album-wait loop (3s idle poll)

**Причина:** `OnEditedMessage` и album/next-link workflows содержат `time.Sleep` циклы, требующие fake clock или `synctest` для контроля таймингов.

**Рекомендация:**
1. `OnEditedMessage` — покрывается моками без time-зависимости, приоритет высокий.
2. `runNextLinkWorkflow` / `processMediaAlbum` — вынести sleep в injectable dependency или использовать `synctest.Test` (уже есть в проекте). Приоритет средний.

---

### internal/repo/ruleset — 70.4%

**Что не покрыто:**
- `WatchContext` — fsnotify watcher goroutine (Write/Create event, watcher-error, context cancellation)
- `Close` — `w.Close()`
- `validate` — ветки: rule-ID с `:` или `,`, negative `From`, negative `To[i]`, `From == To[i]`
- `negateChatIDs` — `Prev`/`Next` type-switch arms

**Причина:** `WatchContext` зависит от OS filesystem events (fsnotify) — интеграционная забота. Ветки валидации требуют специальных невалидных YAML-фикстур, которые не были добавлены.

**Рекомендация:**
1. Добавить YAML-фикстуры для невалидных правил → покрыть `validate`. Просто и высокоприоритетно.
2. `WatchContext` — перенести в integration-слой с реальными temp-файлами. Приоритет низкий.

---

### internal/repo/telegram — 42.0%

**Что не покрыто:** 17+ stub-методов (`SendMessage`, `ForwardMessages`, `GetMessage`, `EditMessageText`, `DeleteMessages`, `GetMessageLink`, `TranslateText` и т.д.), `Start`, `Close`, `Updates`.

**Причина:** все методы — Phase-A заглушки (`return 0, nil`). Тестирование подтверждает только `return nil`.

**Рекомендация:** не тестировать до Phase B (реальная интеграция с TDLib). Покрытие вырастет естественным образом при замене стабов реальной логикой. **Исключение из целевого покрытия.**

---

### internal/repo/term — 0.0%

**Что не покрыто:** `New`, `ReadLine`, `ReadPassword`, `Println`, `Printf`.

**Причина:** `ReadPassword` оборачивает `golang.org/x/term.ReadPassword(fd)` — syscall, который падает без TTY. Авторы пропустили весь файл из-за этого.

**Рекомендация:**
1. `ReadLine`, `Println`, `Printf` — тривиально тестируемы через `strings.NewReader`/`bytes.Buffer`. Добавить.
2. `ReadPassword` — оставить без unit-теста (или выделить интерфейс для injection).

---

### internal/service/facade — 0.0%

**Что не покрыто:** все 12 методов.

**Причина:** тонкий proxy — каждый метод делегирует в `telegramGateway` за 1-2 строки. Единственная логика: `SendMessageAlbum` (итерация + `ContentTypeByFileExt`) и `GetStatus` (`releaseVersion` через `debug.BuildInfo`, возвращает `(nil, false)` в test binary).

**Рекомендация:**
1. `SendMessageAlbum` и `GetStatus` — покрыть, содержат ветвление.
2. Остальные pass-through методы — low value, можно пропустить. **Частичное исключение из целевого покрытия.**

---

### internal/service/message — 61.5%

**Что не покрыто:**
- `BuildInputContent` для `ContentVideo`, `ContentAnimation`, `ContentAudio`
- Default/unknown ветка в `BuildInputContent` switch

**Причина:** пропущенные ветки структурно идентичны покрытым (`ContentPhoto`, `ContentDocument`) — copy-paste логика, не перенесённая в test table.

**Рекомендация:** расширить существующую таблицу `TestBuildInputContent_*` четырьмя кейсами. Ожидаемый прирост: ~90%. Приоритет высокий — минимальные усилия.

---

### internal/service/transform — 46.7%

**Что не покрыто:**
- `replaceMyselfLinks` целиком — `basicGroup` early-return, `TextEntityURL`/`TextEntityTextURL` замена, `DeleteExternal`, `applyReplacement`
- `findCopyLink` — state-store lookup + `GetMessageLink`
- `addAutoAnswer` — `GetCallbackQueryAnswer` success/error/empty
- `addText` fallback при ошибке `ParseTextEntities`
- `formatDstPrefix` — negative ChatID
- Translation failure fallback

**Причина:** `replaceMyselfLinks` + `findCopyLink` требуют сложной настройки моков для цепочки вызовов через `stateStore` и `telegramGateway`. `applyReplacement` делает in-place UTF-16 хирургию — тестируемо изолированно, но пропущено.

**Рекомендация:**
1. `applyReplacement`, `formatDstPrefix`, `addText` fallback — чистые функции, покрыть немедленно.
2. `replaceMyselfLinks` — разбить на более мелкие тестируемые функции или покрыть через integration/BDD.
3. `addAutoAnswer` — покрыть через моки.

---

### internal/transport/term — 51.8%

**Что не покрыто:**
- `Run` — public entrypoint (wires `auth.Subscribe` + `runInputLoop`)
- `processAuth` для `WaitPhone` (interactive `ReadPassword`), `WaitCode`, `Ready`
- `processCommand` unknown command
- `handleHelp`
- Context-cancellation выход из `runInputLoop`

**Причина:** `Run` требует синхронизации goroutine + cancellable context. `processAuth` phone/code пути используют `ReadPassword` (terminal ioctl — та же проблема что у `repo/term`).

**Рекомендация:**
1. `handleHelp`, `processCommand` unknown — тривиально, добавить.
2. `processAuth` для `WaitCode`, `Ready` — не используют `ReadPassword`, покрыть.
3. `processAuth` для `WaitPhone` с `ReadPassword` — выделить интерфейс или пропустить.

---

## Приоритеты

### Quick wins (минимальные усилия, максимальный прирост)

| Пакет | Действие | Ожидаемое покрытие |
|-------|----------|-------------------|
| internal/domain | Табличные тесты для `String`, `DeepCopy`, `ContentTypeByFileExt` | →95% |
| internal/service/message | +4 кейса в `BuildInputContent` table test | →90% |
| internal/controller | +2 теста для `/ready` | →95% |
| internal/repo/ruleset | YAML-фикстуры для невалидных правил | →85% |

### Средний приоритет

| Пакет | Действие | Ожидаемое покрытие |
|-------|----------|-------------------|
| internal/handler | Тест `OnEditedMessage` через моки | →80% |
| internal/service/transform | Тесты `applyReplacement`, `formatDstPrefix`, `addAutoAnswer` | →65% |
| internal/transport/term | Тесты `handleHelp`, `processAuth` WaitCode/Ready | →70% |

### Осознанные исключения

| Пакет | Причина исключения |
|-------|-------------------|
| internal/repo/telegram | Phase-A заглушки; покрытие придёт с Phase B |
| internal/repo/term | `ReadPassword` — terminal syscall; покрыть `ReadLine`/`Printf` |
| internal/service/facade | Тонкий proxy; покрыть только `SendMessageAlbum` и `GetStatus` |
