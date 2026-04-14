# Аудит полноты миграции budva43 → budva-claude

## Методика

Сравнение каждого файла budva43 с budva-claude по функциям, бизнес-логике, эвристикам и edge cases. Последняя актуализация: 2026-04-14.

## Статус Phase A (без CGO) — ✓ ЗАКРЫТА

Все GAPs фазы A реализованы и проверены ревьюерами.

### Закрытые GAPs

| # | Область | Описание | Коммит |
|---|---|---|---|
| G3 | Handler | Retry 3x для edit и delete handlers | `b524f07` |
| G4 | Handler | Rate limiting в forwarding path | `b524f07` |
| G5 | Handler | Statistics counters (viewed + forwarded) | `b524f07` |
| G6 | Handler | Reply chain preservation (resolveReplyTo) | `b524f07` |
| G7 | Handler | Origin message unwrapping (getOriginMessage) | `b524f07` |
| G8 | Transform | addAutoAnswer (callback query injection) | `b524f07` |
| G10 | Config | Fragment UTF-16 length validation | `b524f07` |
| G12 | Handler | Album WithSources — only first message | `b524f07` |
| G13 | Handler | Check/Other dedup через DedupTracker | `b524f07` |
| G14 | Handler | Reply markup sync при edit | `b524f07` |
| G16 | Message | BuildInputContent per-type с LinkPreview inversion | `b524f07` |
| G18 | HTTP | Password hint в state response + Extra() в auth | `b524f07` |

### Были уже реализованы (ложные GAPs в первой версии отчёта)

| # | Область | Почему не GAP |
|---|---|---|
| G9 | Config | Chat ID negation — уже в `ruleset.transform()` |
| G11 | Queue | Panic recovery — уже в `queue.executeTask()` |
| G20 | Config | ErrEmptyConfig — уже в `ruleset.check()` |

### Отложены (не было в budva43 edit handler / Phase B)

| # | Область | Причина |
|---|---|---|
| G15 | Handler | Filter re-check on edit — в budva43 проверка была только для check chat re-forwarding, не полный re-filter |
| G17 | Transform | ForAlbum flag — параметр TDLib API `getMessageLink`, нужен CGO |

## Статус Phase B (TDLib-специфичные) — ОТКРЫТА

| # | Область | Описание | Блокер |
|---|---|---|---|
| G1 | Auth | `AuthorizationStateClosing` — consume without broadcast | CGO |
| G2 | Auth | `CreateClient` infinite retry loop | CGO |
| G19 | Repo | `ParseTextEntities` / `GetMarkdownText` — static client methods | CGO |
| G21 | Auth | `Close()` sleep 1s workaround для TDLib signal.abort | CGO |
| G22 | Repo | `setupClientLog()` — TDLib log redirect to file | CGO |
| G24 | Config | TDLib parameters (UseFileDatabase, etc.) полнота | CGO |

## Статус по областям (актуальный)

### Авторизация

| Функционал | Статус |
|---|---|
| State machine (WaitPhone/Code/Password) | ✓ |
| Subscribe/broadcast + Extra() | ✓ |
| inputChan / authStateChan | ✓ |
| Phone masking | ✓ |
| GetStatus | ✓ |
| AuthorizationStateClosing | Phase B |
| CreateClient retry loop | Phase B |
| Close() sleep workaround | Phase B |

### Handler

| Функционал | Статус |
|---|---|
| OnNewMessage: source check, system delete, filter, forward/copy | ✓ |
| Rate limiting (3s per chat) | ✓ |
| Statistics (viewed/forwarded) | ✓ |
| Reply chain preservation (resolveReplyTo) | ✓ |
| Origin message unwrapping (getOriginMessage) | ✓ |
| Album 3-second wait + WithSources first-only | ✓ |
| Check/Other dedup | ✓ |
| OnEditedMessage: retry 3x, CopyOnce, reply markup sync | ✓ |
| OnDeletedMessages: retry 3x, indelible, cleanup | ✓ |
| OnMessageSendSucceeded: bidirectional mapping | ✓ |
| parseCopyRef helper | ✓ |

### Transform

| Функционал | Статус |
|---|---|
| Translation | ✓ |
| addAutoAnswer | ✓ |
| replaceMyselfLinks | ✓ |
| replaceFragments | ✓ |
| Sign / Link / Prev / Next | ✓ |
| UTF-16 offsets | ✓ |
| Span instrumentation | ✓ |
| ForAlbum flag | Phase B |

### Message / Filters / Dedup / Album / Limiter / Queue / State / Ruleset

Все — ✓ полностью реализованы.

### Transports

| Транспорт | Статус |
|---|---|
| Terminal (auth + CLI) | ✓ |
| HTTP (REST auth + GraphQL + playground) | ✓ |
| gRPC (FacadeGRPC 10 RPC + reflection) | ✓ |

### DTO

| Слой | Статус |
|---|---|
| `internal/dto/graphql/` | ✓ StatusResponse |
| gRPC proto (pb/) | ✓ Все message types |

## Тестовое покрытие — КРИТИЧЕСКИЕ ПРОБЛЕМЫ

Подробный разбор в `REPORT_TEST.md`. Ключевые проблемы:

### BDD: 18 сценариев — заглушки

Step definitions не вызывают реальный код. Тесты "проходят" вне зависимости от корректности реализации. 7 новых бизнес-функций не покрыты BDD.

### Integration: 0 тестов

Директория пуста. Нет тестов для handler + services pipeline, BadgerDB update-in-place, ruleset full cycle.

### E2E: не существует

Директория отсутствует. Нет тестов lifecycle cmd/engine и cmd/facade.

### Smoke: 0 тестов

Директория пуста. Нет проверки бинарной сборки, загрузки конфига, health endpoint.

## Дальнейший план

### Приоритет 1: Functional BDD (red→green)

Подключить реальный Handler + services в BDD step definitions через `test/support/`. Сделать 18 существующих сценариев функциональными.

### Приоритет 2: Новые BDD-сценарии

7 features для: retry, rate limiting, reply chain, origin unwrapping, statistics, check/other dedup, auto-answer.

### Приоритет 3: Integration тесты

BadgerDB operations, ruleset full pipeline, handler + real services.

### Приоритет 4: E2E с TDLib stubs

Engine lifecycle, facade endpoints.

### Приоритет 5: Smoke тесты

Binary build, config load, health endpoint.

### Приоритет 6: Phase B — TDLib интеграция

CGO + go-tdlib, реальные вызовы, AuthorizationStateClosing, retry loop, setupClientLog.
