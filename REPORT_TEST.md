# Аудит тестового покрытия budva-claude

Последняя актуализация: 2026-04-14.

## Сводка

| Слой | Расположение | Количество | Статус |
|---|---|---|---|
| Unit | `*_test.go` рядом с кодом | 131 тест, 18 пакетов | ✓ |
| BDD | `test/bdd/` | 93 сценария, 25 features | ✓ Функциональные |
| Integration | `test/integration/` | 4 подтеста | ✓ |
| Smoke | `test/smoke/` | 4 теста | ✓ (build tag `smoke`) |
| E2E | `test/e2e/` | — | Отложено (Phase B) |

## Unit-тесты — по пакетам

| Пакет | Тестов | Что проверяется |
|---|---|---|
| config | 3 | envconfig loading, defaults |
| controller | 3 | healthcheck, live, unhealthy |
| domain | 13 | MaskPhoneNumber (11 сценариев + property-based длина) |
| handler | 12 | OnNewMessage (7 сценариев), OnEditedMessage, OnDeletedMessages (retry, indelible), OnMessageSendSucceeded, SetRuleSet |
| repo/queue | 4 | Add, processQueue, panic recovery, StartContext |
| repo/ruleset | 6 | YAML load, validate, negation, UTF-16 fragment validation, full pipeline |
| repo/state | 11 | BadgerDB get/set/delete/atomic, copies (single/multi/update-in-place), bidirectional mapping, increment counters, answer message ID |
| service/album | 10 | AddMessage, PopMessages, LastReceivedAge, MakeKey |
| service/auth | 8 | Subscribe, SetState, Extra, InputChan, ReadInput, concurrency |
| service/dedup | 3 | TryMark, dedup per destination |
| service/limiter | 1 | WaitForForward (synctest: первый вызов — 0s, второй — 3s) |
| service/filters | 8 | Evaluate: exclude, include, submatch, empty text |
| service/message | 7 | GetFormattedText, IsSystemMessage, GetReplyMarkupData, BuildInputContent (4 типа) |
| service/transform | 16 | Transform pipeline, AddNextLink, UTF-16 encode/decode, replaceFragment |
| transport/grpc | 18 | FacadeGRPC все RPC + helpers + error cases (mockery EXPECT) |
| transport/http | 12 | REST auth endpoints (state, phone, code, password, hint) |
| transport/http/graph | 5 | GraphQL handler, status query/error, invalid body, unknown query, gqlgen playground |
| transport/term | 3 | runInputLoop exit command (synctest), processAuth WaitPassword (with/without hint) |

## BDD-тесты — 93 сценария (25 features)

Step definitions вызывают реальный Handler + services через `test/support/Stack` с FakeTelegram (in-memory stateful gateway) и BadgerDB (TempDir).

### Покрытые бизнес-эпики

| Эпик | Features | Сценариев | Описание |
|---|---|---|---|
| 01_DELIVERY | copy, forward, rate limiting, reply chain, origin unwrapping, statistics | ~20 | Пересылка, копирование, rate limit, reply chain, origin unwrap, статистика |
| 02_FILTERS | exclude, include, submatch, check/other dedup | ~15 | Фильтрация по паттернам, дедупликация |
| 03_TRANSFORM | replace links, remove external, fragments, source link, sign, translate | ~20 | Трансформация текста и ссылок |
| 04_MEDIA | album copy, album forward | ~10 | Медиа-альбомы |
| 05_SYNC | versioning, edit update, indelible, delete sync, retry | ~20 | Синхронизация изменений с retry |
| 06_AUTO | auto answers | ~8 | Автоматические ответы (частично @pending — ждёт TDLib) |

## Integration-тесты — 4 подтеста

`test/integration/suite_test.go` — cross-component pipeline:

| Подтест | Что проверяется |
|---|---|
| copy_with_transform | handler → state → transform → forward полный цикл |
| edit_sync | edit propagation через state mapping |
| delete_sync | delete propagation |
| filter_exclude | фильтрация на уровне pipeline |

BadgerDB через TempDir (без testcontainers). FakeTelegram через `telegramGateway` interface.

## Smoke-тесты — 4 теста

`test/smoke/smoke_test.go` (build tag `smoke`):

| Тест | Что проверяется |
|---|---|
| TestHealthcheck | `/healthcheck` endpoint → 200 |
| TestHealth | `/health` endpoint → 200 |
| TestLive | `/live` endpoint → 200 |
| TestReady | `/ready` endpoint → 200 |

Запускаются через testcontainers-compose + Dockerfile. Поднимают реальный стек в Docker.

## E2E — отложено

Требует TDLib Test DC (Phase B). Запланировано:
- Engine lifecycle (startup → process updates → shutdown)
- Facade endpoints (HTTP + gRPC functional)

## Моки

Все моки генерируются через mockery v3 (`.mockery.yml`). Используется EXPECT() API.

| Пакет | Интерфейсы |
|---|---|
| handler/mocks | telegramGateway, stateStore, messageService, filterService, transformService, DedupTracker, albumService, taskQueue, rateLimiter |
| transform/mocks | telegramGateway, stateStore |
| controller/mocks | pinger |
| transport/http/mocks | authService |
| transport/grpc/mocks | facadeService |
| transport/http/graph/mocks | statusProvider |

Inline моки не используются — все заменены на mockery.

## Требования к запуску

Для тестов, использующих `testing/synctest` (limiter, term), требуется `GOEXPERIMENT=synctest`. Taskfile настроен автоматически (`task test`, `task test-short`, `task test-v`).
