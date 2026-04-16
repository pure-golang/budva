# Phase B: промежуточный отчёт и план дальнейших работ

Последняя актуализация: 2026-04-17.

## Что сделано

### Инфраструктура (T1–T3)

- **Dockerfile** — 3-stage build: `tdlib-builder` (Debian bookworm + TDLib C++) → Go builder (CGO_ENABLED=1) → runtime (bookworm-slim + libtd*)
- **go-tdlib v0.7.6** — добавлен как direct dependency; TDLib 1.8.40 собран локально в `/usr/local`
- **Config** — 8 новых полей в `TelegramConfig` (UseFileDatabase, UseChatInfoDatabase, UseMessageDatabase, UseSecretChats, SystemVersion, ApplicationVersion, LogDirectory, LogMaxFileSize); убран `UseTestDC`
- **.env.example** и **doc.go** — синхронизированы с config

### TDLib-клиент (T4–T11)

- **repo/telegram/repo.go** — `Repo` с `*client.Client`, auth loop через `clientAuthorizer`, `listenAuthStates` с фильтрацией transitional states (WaitTdlibParameters → пропускается), `listenUpdates` для NewMessage/SendSucceeded/DeleteMessages
- **repo/telegram/mapping.go** — полный маппинг domain ↔ go-tdlib: `mapMessage`, `mapMessageContent` (Text/Photo/Video/Document/Audio/Animation/VoiceNote), `mapForwardInfo`, `mapReplyTo`, `mapReplyMarkup`, `toTDLibInputMessageContent`, `toTDLibFormattedText`
- **repo/telegram/client_adapter.go** — 18 методов: Send/Forward/Edit/Delete/Get сообщений, LoadChats, WarmUpChat, GetChatHistory, GetChatType, ParseTextEntities, GetMarkdownText, TranslateText, GetCallbackQueryAnswer, GetOption, GetMe, Create/Delete чатов, SetSupergroupUsername
- **Close()** — пропускает явный `client.Close()` из-за гонки в go-tdlib v0.7.6 (SIGABRT из C++ runtime)

### cmd/stand

- **--up** — дозаполнение: пропускает существующие чаты, создаёт только недостающие, сохраняет partial results при ошибке
- **--down** — сохраняет неудалённые чаты в fixtures для повторного запуска
- **Паузы** — рандомный jitter 3–8 сек между API-вызовами (rate limit mitigation)
- **8 тестовых чатов** — 4 исходных (SRC PUB CHL, SRC PRV CHL, SRC PUB GRP, SRC PRV GRP) + 4 целевых (DST PUB CHL, DST PRV CHL, DST PUB GRP, DST PRV GRP)

### BDD-инфраструктура

- **LiveStack** — собранный стек с реальным TDLib; один экземпляр на все сценарии (TDLib не пересоздаётся); `ResetState()` пересоздаёт BadgerDB, Queue и Handler между сценариями
- **Nanoid prefix** — 8-символьный hex-маркер на каждый сценарий; `PutMessage` добавляет prefix к тексту; `CheckLastMessage` поллит целевой чат до 10 сек и проверяет prefix последнего сообщения; `CheckNoMessage` ждёт 3 сек и проверяет отсутствие prefix
- **Удалены** — `FakeTelegram`, `Stack`, `auth_flow_test.go`, `CleanupChat`
- **Feature-файлы** — 18 файлов обновлены: переименованы чаты ("публичный канал" → "исходный публичный канал"), добавлены source→target сценарии (матрица 4×4)

### Текущие результаты

| Метрика | Значение |
|---|---|
| `go build` | OK |
| `task lint` | 0 issues |
| Unit-тесты (`go test -short ./internal/...`) | все проходят |
| BDD-сценарии | **257 из 443 проходят** |
| BDD-сценарии failed | 1 (source_link — зависит от permalink) |
| BDD-сценарии undefined | 185 (failfast после первого failure) |

## Что не работает и почему

### 1. Temporary ID vs Permanent ID

**Проблема:** `Repo.SendMessage` возвращает temporary message ID. TDLib присваивает permanent ID асинхронно через `UpdateMessageSendSucceeded`. Функции, зависящие от permanent ID, не работают:

- `GetMessageLink(chatID, tempMsgID)` → 404 (нет permalink для temporary ID)
- `GetMessage(chatID, tempMsgID)` → 404 (TDLib не индексирует по temporary ID)

**Затронутые features:**
- `03_transform/04_source_link.feature` — "К копии добавляется ссылка на оригинал"
- `05_sync/01_versioning.feature` — "Версионирование" (GetMessageLink для Prev/Next)
- Reply chain в `01_delivery` — handler строит reply по temporary ID

**Решение:** после `SendMessage` нужно дождаться `UpdateMessageSendSucceeded` и заменить temporary ID на permanent. Варианты:
1. Добавить `WaitForPermanentID(ctx, chatID, tempID) (permanentID, error)` в Repo — слушает update channel
2. Или: `SendMessageAndWait` который блокирует до получения permanent ID
3. Или: в BDD-тестах использовать паттерн budva43 — отправлять через gRPC facade (который уже обрабатывает temp→permanent внутри)

### 2. go-tdlib v0.7.6 race condition

**Проблема:** глобальный `tdlib.receiver` отправляет в `client.responses` канал, который закрывается при `AuthorizationStateClosed` → panic "send on closed channel" / SIGABRT при Close.

**Текущий workaround:** `Repo.Close()` не вызывает `client.Close()`, TDLib завершает сессию при выходе процесса.

**Долгосрочное решение:** форк go-tdlib с фиксом гонки, или переход на другую Go-обёртку TDLib.

### 3. 185 undefined BDD-сценариев

**Проблема:** godog с `Strict: true` + `-failfast` останавливается после первого failure. Все последующие сценарии помечаются undefined.

**Решение:** исправить единственный falling scenario (source_link) — остальные 185 запустятся.

## План дальнейших работ

### Фаза 1: Permanent ID (критичная)

1. Добавить в `Repo` метод `SendMessageAndWait(ctx, chatID, content) (*domain.Message, error)`:
   - Вызывает `tdClient.SendMessage`
   - Подписывается на updates через `tdClient.GetListener`
   - Ждёт `UpdateMessageSendSucceeded` с matching `OldMessageId`
   - Возвращает полный `*domain.Message` с permanent ID
   - Timeout: 30 сек

2. Обновить `LiveStack.PutMessage` — использовать `SendMessageAndWait`

3. Обновить handler flow — `OnMessageSendSucceeded` уже обрабатывается; нужно чтобы BDD-тесты вызывали его с правильными ID после реальной отправки

### Фаза 2: Permalink-зависимые features

1. Исправить `source_link` feature — после permanent ID GetMessageLink будет работать
2. Исправить `versioning` feature — Prev/Next ссылки через permanent ID
3. Исправить `reply chain` — handler строит reply по permanent ID
4. Ожидаемый результат: 443/443 BDD-сценариев проходят

### Фаза 3: Стабилизация

1. Убрать `-failfast` из `task test` для BDD (чтобы видеть все failures, а не только первый)
2. Добавить BDD в CI (с условием: TDLib + .env + stand.json)
3. Рассмотреть форк go-tdlib для фикса race condition при Close
4. Обновить `docs/TEST-MATRIX.md`

### Фаза 4: Опциональные улучшения

1. Docker-based BDD (TDLib в контейнере, credentials через secrets)
2. Rate limit handling в Repo (retry с backoff при 429)
3. `UpdateMessageEdited` маппинг в update listener (сейчас не обрабатывается)
4. Полная матрица source→target для всех feature-файлов (сейчас 16 пар только в delivery + transform + filters)

## Известные ограничения

| Ограничение | Митигация |
|---|---|
| TDLib собирается ~30 мин | Docker кеширует stage; локально собирается один раз |
| BDD зависят от реального Telegram | Отдельный аккаунт, stand --up для чатов |
| go-tdlib Close() вызывает SIGABRT | Пропускаем Close(), TDLib чистится при exit |
| Temporary ID не имеет permalink | Фаза 1: SendMessageAndWait |
| Rate limits при создании чатов | Рандомные паузы 3–8 сек + partial save |
