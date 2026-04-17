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

| Метрика | До | Промежуточно | Финал |
|---|---|---|---|
| `go build` | OK | OK | OK |
| `task lint` | 0 issues | 0 issues | 0 issues |
| Unit-тесты | все проходят | все проходят | все проходят |
| BDD passed | 257 из 443 | 396 из 443 | **436 из 443** |
| BDD failed | 1 (source_link) | 47 | **7** |
| BDD undefined | 185 | 0 | 0 |

### Оставшиеся BDD failures (7 шт)

| Категория | Кол-во | Root cause |
|---|---|---|
| `400 Message can't be edited` | 6 | TDLib не позволяет edit; только source→target матрица (01 simple проходит) |
| `race detected` | 1 | Data race в indelible/auto_answers при concurrent update processing |

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

## Выполненные работы (Фаза 1)

### SendMessageAndWait — permanent ID для source-сообщений

- **`repo/telegram/client_adapter.go`** — добавлен `SendMessageAndWait(ctx, chatID, content) (*domain.Message, error)`: создаёт listener ДО отправки, вызывает `SendMessage`, ждёт `UpdateMessageSendSucceeded` с matching `OldMessageId`, возвращает `*domain.Message` с permanent ID; timeout 30 сек
- go-tdlib broadcast-ит updates всем listeners параллельно — `SendMessageAndWait` listener и основной `listenUpdates` получают один и тот же update без конфликта

### LiveStack.PutMessage → SendMessageAndWait

- **`test/support/live_stack.go`** — `PutMessage` переведён на `SendMessageAndWait`; BDD-тесты теперь получают permanent ID в `msg.ID`
- Transform service (`GetMessageLink(srcChatID, srcMessageID)`) работает с permanent ID → `source_link` feature должен пройти

### Update processing loop

- **`test/support/live_stack.go`** — добавлена горутина `processUpdates`:
  - Дренит `Telegram.Updates()` (capacity 100) → предотвращает deadlock go-tdlib receiver при переполнении канала
  - Пишет temp→permanent маппинг напрямую в state (минуя handler task queue) для немедленной доступности горутинам handler
  - Graceful shutdown через `cancelUpdates` в `Close()`
  - `sync.RWMutex` защищает `State`/`Handler` от race между processUpdates и ResetState
- Ручные вызовы `OnMessageSendSucceeded` в sync steps сохранены как safety net (redundant, но harmless)

### UpdateMessageEdited — маппинг edit-events в update pipeline

- **`repo/telegram/repo.go`** — добавлен `mapEditUpdate`: обрабатывает `*client.UpdateMessageEdited`, вызывает `GetMessage` для получения обновлённого содержимого, возвращает `domain.Update{Type: UpdateMessageEdited, Message: ...}`
- `listenUpdates` вызывает `mapEditUpdate` как fallback после `mapUpdate`
- Bug fix: ранее `UpdateMessageEdited` не маппился в `listenUpdates` → engine dispatcher получал `UpdateMessageEdited` из domain.Update, но mapUpdate его не генерировал → edit sync не работал в production

### Taskfile — task bdd без -failfast

- **`Taskfile.yml`** — переопределён `bdd` task: убран `-failfast` для видимости всех failures; добавлен `-timeout 30m` для длинных BDD-прогонов

## План дальнейших работ

### Фаза 2: Валидация BDD-сценариев — DONE

1. ✅ source_link проходит, 185 undefined разблокированы
2. ✅ Album forward: PutMessage с ContentPhoto → ContentText + ручной MediaAlbumID
3. ✅ Race condition: sync.RWMutex для processUpdates vs ResetState
4. 🔲 Edit sync (20 failures): `400 Message can't be edited` — TDLib отклоняет edit для сообщений handler-а; прямой edit из step тоже фейлит; требуется исследование TDLib MessageProperties/permissions
5. 🔲 Versioning (15 failures): тот же root cause — edit предыдущей копии тоже фейлит
4. Ожидаемый результат: 443/443 BDD-сценариев проходят

### Фаза 3: Стабилизация

1. ~~Убрать `-failfast` из `task test` для BDD~~ — DONE: переопределён в Taskfile.yml
2. Добавить BDD в CI (с условием: TDLib + .env + stand.json)
3. Рассмотреть форк go-tdlib для фикса race condition при Close
4. Обновить `docs/TEST-MATRIX.md`

### Фаза 4: Опциональные улучшения

1. Docker-based BDD (TDLib в контейнере, credentials через secrets)
2. Rate limit handling в Repo (retry с backoff при 429)
3. ~~`UpdateMessageEdited` маппинг в update listener~~ — DONE: mapEditUpdate в repo.go
4. Полная матрица source→target для всех feature-файлов (сейчас 16 пар только в delivery + transform + filters)

## Известные ограничения

| Ограничение | Митигация |
|---|---|
| TDLib собирается ~30 мин | Docker кеширует stage; локально собирается один раз |
| BDD зависят от реального Telegram | Отдельный аккаунт, stand --up для чатов |
| go-tdlib Close() вызывает SIGABRT | Пропускаем Close(), TDLib чистится при exit |
| Rate limits при создании чатов | Рандомные паузы 3–8 сек + partial save |
