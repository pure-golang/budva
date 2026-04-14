# Аудит полноты миграции budva43 → budva-claude

## Методика

Сравнение каждого файла budva43 с budva-claude по функциям, бизнес-логике, эвристикам и edge cases.

## Статус по областям

### 1. Авторизация

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| State machine (WaitPhone/Code/Password) | ✓ | ✓ | OK |
| Subscribe/broadcast | goroutine per subscriber | sync callback | OK (упрощено) |
| `AuthorizationStateClosing` consumed but NOT broadcast | ✓ | ✗ Не учтено | **GAP** |
| `CreateClient` infinite retry loop (no delay, no limit) | ✓ | ✗ Нет retry | **GAP** |
| `Close()` с sleep 1s (workaround TDLib signal.abort) | ✓ | ✗ Нет workaround | **GAP** |
| `inputChan` buffered size 1 | ✓ | ✓ | OK |
| `authStateChan` buffered size 10 + non-blocking send | ✓ | ✓ | OK |
| Phone number masking в выводе | `MaskPhoneNumber()` | `phone[:4]***` | OK (упрощено) |
| GetStatus (version + userId) | ✓ | ✓ | OK |

### 2. Telegram Repo

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| `getClient()` — блокирующее ожидание авторизации | ✓ | `ClientDone()` channel | OK (аналог) |
| `setupClientLog()` — TDLib log to file | ✓ | ✗ Нет | **GAP** |
| `LoadChats` / `GetChatHistory` — проверка `client == nil` | ✓ | ✗ Нет (stubs) | Ждёт TDLib |
| `ParseTextEntities` — static client method (не через getClient) | ✓ | ✗ Не учтено | **GAP** |
| `GetMarkdownText` — static client method | ✓ | ✗ Отсутствует | **GAP** |
| `GetChat` — тип чата | ✓ | `GetChatType` stub | OK |
| `GetListener` — TDLib update listener | ✓ | channel-based updates | OK (другая архитектура) |
| TDLib parameters: UseFileDatabase, UseChatInfoDatabase, UseMessageDatabase, UseSecretChats | ✓ | Частично в config | **CHECK** |

### 3. Handler — OnNewMessage

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| Source check + system message deletion | ✓ | ✓ | OK |
| Filter evaluation (OK/Check/Other) | ✓ | ✓ | OK |
| Forward without copy | ✓ | ✓ | OK |
| Send copy + transform | ✓ | ✓ | OK |
| Media album 3-second wait | рекурсивный timer с `GetLastReceivedDiff` | `processMediaAlbum` с for-loop | OK |
| Album source attribution only for FIRST message | `withSources` flag | `WithSources: true` всегда | **GAP** |
| **Check/Other dedup через map** | `forwardedTo` map prevents duplicate check/other | ✗ Нет dedup для check/other | **GAP** |
| **Statistics** (viewed + forwarded counters) | `addStatistics()` в конце | ✗ Отсутствует | **GAP** |
| **Rate limiting** перед каждым forward | `WaitForForward(3s per chat)` | ✗ Отсутствует | **GAP** |
| **Origin message unwrapping** (channel forwards) | `getOriginMessage()` | ✗ Отсутствует | **GAP** |
| **Reply chain preservation** | `getReplyToMessageId()` | ✗ Отсутствует | **GAP** |

### 4. Handler — OnEditedMessage

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| Edit propagation to all copies | ✓ | ✓ | OK |
| **Retry 3 times** (eventual consistency) | `maxRetry=3`, queue re-insertion | ✗ Нет retry | **GAP** |
| CopyOnce versioning (new message + prevLink) | ✓ | ✓ | OK |
| **Reply markup sync** (answer message tracking) | `SetAnswerMessageId` / `DeleteAnswerMessageId` | ✗ Вызывается в delete, но НЕ в edit | **GAP** |
| **Auto-answer** injection при edit | через transform | ✗ Нет auto-answer | **GAP** |
| **Filter re-check** on edit (re-forward to check chat) | ✓ | ✗ Нет re-check | **GAP** |

### 5. Handler — OnDeletedMessages

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| Permanent-only check | ✓ | ✓ | OK |
| Indelible rule skip | ✓ | ✓ | OK |
| Storage cleanup order | ✓ | ✓ | OK |
| **Retry 3 times** (eventual consistency) | `maxRetry=3`, queue re-insertion | ✗ Нет retry | **GAP** |

### 6. Handler — OnMessageSendSucceeded

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| Bidirectional mapping (tmp↔new) | ✓ | ✓ | OK |

### 7. Transform Service

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| Translation | ✓ | ✓ | OK |
| **addAutoAnswer** (callback query injection) | ✓ | ✗ Отсутствует | **GAP** |
| replaceMyselfLinks | ✓ | ✓ | OK |
| replaceFragments | ✓ | ✓ | OK |
| Source sign | ✓ | ✓ | OK |
| Source link | ✓ | ✓ | OK |
| Prev link | ✓ | ✓ | OK |
| Next link | ✓ | ✓ | OK |
| **ForAlbum flag** in GetMessageLink | ✓ | ✗ Не передаётся | **GAP** |
| UTF-16 offset management | ✓ | ✓ | OK |
| Markdown v2 parsing | ✓ | ✓ | OK |

### 8. Message Service

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| GetFormattedText | ✓ | ✓ | OK |
| IsSystemMessage (7 system types) | ✓ | Упрощённый (по ContentSystem) | **CHECK** |
| GetReplyMarkupData | ✓ | ✓ | OK |
| **GetInputMessageContent** — LinkPreviewOptions | ✓ | ✗ Упрощённый BuildInputContent | **GAP** |
| **GetInputMessageContent** — Thumbnail handling | ✓ | ✗ Не передаёт Thumbnail | **GAP** |
| **GetInputMessageContent** — per-content-type logic | Полный switch (7 типов) | Плоское копирование полей | **GAP** |

### 9. Filters Service

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| Exclude regex | ✓ | ✓ | OK |
| Include regex | ✓ | ✓ | OK |
| IncludeSubmatch с group extraction | ✓ | ✓ | OK |
| Empty text + include → FiltersOther | ✓ | ✓ | OK |
| Case-insensitive (?i) | ✓ | ✓ | OK |

### 10. Dedup Service

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| TryMark dedup per destination | ✓ | ✓ | OK |
| **Init map с destinations** перед processing | `forwardedTo.Init()` | ✗ Tracker не pre-init | **MINOR** |

### 11. Rate Limiter

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| **3-second per-chat throttle** | `WaitForForward()` | ✓ Реализован (Service struct) | OK |
| **Интеграция** в forwarding path | Вызывается в forwarder | ✗ Не вызывается в handler | **GAP** |

### 12. Album Service

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| AddMessage / PopMessages / LastReceivedAge | ✓ | ✓ | OK |
| MakeKey (ruleId:albumId) | ✓ | ✓ | OK |

### 13. Storage (repo/state)

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| copiedMessageIds (CSV, update-in-place) | ✓ | ✓ | OK |
| newMessageId / tmpMessageId (bidirectional) | ✓ | ✓ | OK |
| answerMessageId | ✓ | ✓ | OK |
| viewedMessages / forwardedMessages counters | ✓ | ✓ | OK |
| Increment (atomic merge operator) | ✓ | ✓ | OK |
| GetSet (atomic read-modify-write) | ✓ | ✓ | OK |
| GC (5 min interval, threshold 0.7) | ✓ | ✓ | OK |

### 14. Queue (repo/queue)

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| **1-second tick** per task execution | ✓ | ✗ Нет tick, задачи через `Add()` | **CHECK** |
| **Panic recovery** в executeTask | ✓ | ✗ Нет recovery | **GAP** |
| FIFO (linked list) | ✓ | Slice-based | OK |

### 15. Config / Ruleset

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| YAML loading | Viper | yaml.v3 | OK |
| File watching (fsnotify) | ✓ | ✓ | OK |
| **Chat ID negation** (transform step) | Все ID негируются | ✗ Нет negation | **GAP** |
| **Fragment UTF-16 length validation** | `validate()` проверяет From/To length | ✗ Нет валидации | **GAP** |
| Enrich (UniqueSources, UniqueDestinations, OrderedRules) | ✓ | ✓ | OK |
| ErrEmptyConfigData | ✓ | ✗ Нет проверки | **GAP** |

### 16. Loader (warm-up)

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| `LoadChats(200)` при старте | ✓ | ✗ Stub, не вызывается | Ждёт TDLib |
| `GetChatHistory(limit=1)` для каждого destination | ✓ | ✗ Stub, не вызывается | Ждёт TDLib |

### 17. Transport — Terminal

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| Auth flow (phone/code/password) | ✓ | ✓ | OK |
| CLI commands (help, exit) | ✓ | ✓ | OK |
| **HiddenReadLine** для secrets | ✓ | `ReadPassword()` | OK |
| Phone masking | `MaskPhoneNumber()` | `phone[:4]***` | OK |
| **Русский UI** (Введите номер телефона) | ✓ | Английский | **MINOR** |

### 18. Transport — HTTP

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| REST auth endpoints (state/phone/code/password) | ✓ | ✓ | OK |
| GraphQL handler | gqlgen + generated code | Лёгкий handler (status only) | OK (MVP) |
| **GraphQL playground** | ✓ | ✓ | OK |
| **Password hint в state response** | PasswordHint передаётся | ✗ Не передаётся | **GAP** |

### 19. Transport — gRPC

| Функционал | budva43 | budva-claude | Статус |
|---|---|---|---|
| FacadeGRPC (10 RPC) | ✓ | ✓ | OK |
| Reflection | ✓ | ✓ | OK |
| GetChatHistory | ✓ | ✓ | OK |

---

## Сводка GAPs

### Критические (блокируют корректную работу при подключении TDLib)

| # | Область | Описание |
|---|---|---|
| G1 | Auth | `AuthorizationStateClosing` не обрабатывается (consume without broadcast) |
| G2 | Auth | `CreateClient` retry loop отсутствует |
| G3 | Handler | Retry 3x для edit и delete handlers (eventual consistency) |
| G4 | Handler | Rate limiting не вызывается при forwarding |
| G5 | Handler | Statistics counters не инкрементируются |
| G6 | Handler | Reply chain preservation отсутствует |
| G7 | Handler | Origin message unwrapping отсутствует |
| G8 | Transform | `addAutoAnswer` отсутствует |
| G9 | Config | Chat ID negation отсутствует |
| G10 | Config | Fragment UTF-16 length validation отсутствует |
| G11 | Queue | Panic recovery отсутствует |

### Средние (влияют на корректность edge cases)

| # | Область | Описание |
|---|---|---|
| G12 | Handler | Album source attribution — `WithSources` для первого сообщения |
| G13 | Handler | Check/Other dedup через forwardedTo map |
| G14 | Handler | Reply markup sync при edit (SetAnswerMessageId) |
| G15 | Handler | Filter re-check при edit |
| G16 | Message | `BuildInputContent` не учитывает LinkPreviewOptions, thumbnails, per-type logic |
| G17 | Transform | ForAlbum flag в GetMessageLink |
| G18 | HTTP | Password hint в state response |
| G19 | Repo | `ParseTextEntities` / `GetMarkdownText` — static client methods |
| G20 | Config | `ErrEmptyConfigData` проверка при пустом конфиге |

### Низкие (cosmetic, workarounds)

| # | Область | Описание |
|---|---|---|
| G21 | Auth | `Close()` sleep 1s workaround для TDLib |
| G22 | Repo | `setupClientLog()` — TDLib log redirect to file |
| G23 | Terminal | Русский UI (Введите номер телефона) |
| G24 | Config | TDLib parameters (UseFileDatabase, etc.) полнота |

---

## Рекомендуемый порядок закрытия GAPs

### Фаза A: Подготовка к TDLib (без CGO)

Эти GAPs можно закрыть до подключения TDLib, используя существующие stubs.

1. **G9** — Chat ID negation в ruleset (validate/transform/enrich)
2. **G10** — Fragment UTF-16 length validation
3. **G20** — ErrEmptyConfigData check
4. **G3** — Retry 3x для edit/delete handlers
5. **G4** — Rate limiter интеграция в handler
6. **G5** — Statistics counters
7. **G6** — Reply chain preservation
8. **G7** — Origin message unwrapping
9. **G8** — addAutoAnswer в transform
10. **G11** — Panic recovery в queue
11. **G12** — Album WithSources flag
12. **G13** — Check/Other dedup
13. **G14** — Reply markup sync при edit
14. **G15** — Filter re-check при edit
15. **G16** — BuildInputContent per-type logic
16. **G17** — ForAlbum flag
17. **G18** — Password hint в HTTP state response

### Фаза B: TDLib-специфичные (требуют CGO)

1. **G1** — AuthorizationStateClosing handling
2. **G2** — CreateClient retry loop
3. **G19** — Static client methods (ParseTextEntities)
4. **G21** — Close() sleep workaround
5. **G22** — setupClientLog()
6. **G24** — TDLib parameters полнота
