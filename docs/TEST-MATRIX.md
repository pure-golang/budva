# Test Matrix

Последняя актуализация: 2026-04-14.

## Сводка по слоям

| Слой | Пакетов | Тестов | Статус |
|---|---|---|---|
| unit | 18 | 131 | ✅ green |
| integration | 1 | 4 subtest | ✅ green |
| bdd | 1 | 27 scenarios | ✅ green |
| e2e | 0 | 0 | — (Phase B) |
| smoke | 1 | 4 | ✅ green (build tag `smoke`) |

Запуск: `task test` (включает `GOEXPERIMENT=synctest`).

## Unit-тесты по пакетам

| Пакет | Тестов | Что покрывается |
|---|---|---|
| internal/config | 3 | envconfig loading, defaults |
| internal/controller | 3 | healthcheck, live, unhealthy |
| internal/domain | 13 | MaskPhoneNumber (11 сценариев + property-based) |
| internal/handler | 12 | OnNewMessage (7), OnEditedMessage, OnDeletedMessages (retry, indelible), OnMessageSendSucceeded, SetRuleSet |
| internal/repo/queue | 4 | Add, processQueue, panic recovery, StartContext |
| internal/repo/ruleset | 6 | YAML load, validate, negation, UTF-16 fragment, full pipeline |
| internal/repo/state | 11 | BadgerDB CRUD, copies (single/multi/update-in-place), bidirectional mapping, counters |
| internal/repo/telegram | 2 | RunAuthFlow full cycle (synctest), cancel during input |
| internal/service/album | 10 | AddMessage, PopMessages, LastReceivedAge, MakeKey |
| internal/service/auth | 8 | Subscribe, SetState, Extra, InputChan, ReadInput, concurrency |
| internal/service/dedup | 3 | TryMark, dedup per destination |
| internal/service/filters | 8 | Evaluate: exclude, include, submatch, empty text |
| internal/service/limiter | 1 | WaitForForward (synctest: 0s first, 3s second) |
| internal/service/message | 7 | GetFormattedText, IsSystemMessage, GetReplyMarkupData, BuildInputContent |
| internal/service/transform | 16 | Transform pipeline, AddNextLink, UTF-16, replaceFragment |
| transport/grpc | 18 | FacadeGRPC все RPC + helpers + error cases (mockery EXPECT) |
| transport/http | 12 | REST auth endpoints (state, phone, code, password, hint) |
| transport/http/graph | 5 | GraphQL handler, status query/error, invalid body, unknown query, gqlgen playground |
| transport/term | 3 | runInputLoop exit (synctest), processAuth WaitPassword (with/without hint) |

## BDD-сценарии

| Эпик | Файл | Сценариев | Статус |
|---|---|---|---|
| 01_delivery | 01_copy.feature | 4 | ✅ |
| 01_delivery | 02_forward.feature | 4 | ✅ |
| 01_delivery | 03_rate_limiting.feature | 1 | ✅ |
| 01_delivery | 04_reply_chain.feature | 1 | ✅ |
| 01_delivery | 05_origin_unwrapping.feature | 1 | ✅ |
| 01_delivery | 06_statistics.feature | 1 | ✅ |
| 01_delivery | 07_system_messages.feature | 2 | ✅ |
| 02_filters | 01_exclude.feature | 16 | ✅ |
| 02_filters | 02_include.feature | 8 | ✅ |
| 02_filters | 03_submatch.feature | 8 | ✅ |
| 02_filters | 04_check_other_dedup.feature | 1 | ✅ |
| 03_transform | 01_replace_own_links.feature | 3 | ✅ |
| 03_transform | 02_remove_external_links.feature | 3 | ✅ |
| 03_transform | 03_replace_fragments.feature | 4 | ✅ |
| 03_transform | 04_source_link.feature | 3 | ✅ |
| 03_transform | 05_source_sign.feature | 4 | ✅ |
| 03_transform | 06_translate.feature | 4 | ✅ |
| 04_media | 01_album_copy.feature | 4 | ✅ |
| 04_media | 02_album_forward.feature | 4 | ✅ |
| 05_sync | 01_versioning.feature | 3 | ✅ |
| 05_sync | 02_edit_update.feature | 4 | ✅ |
| 05_sync | 03_indelible.feature | 4 | ✅ |
| 05_sync | 04_delete_sync.feature | 4 | ✅ |
| 05_sync | 05_retry_eventual_consistency.feature | 2 | ✅ |
| 06_auto | 01_auto_answers.feature | 3 | ✅ |

## Integration-тесты

| Подтест | Что проверяется |
|---|---|
| copy_with_transform | handler → state → transform → forward полный цикл |
| edit_sync | edit propagation через state mapping |
| delete_sync | delete propagation |
| filter_exclude | фильтрация на уровне pipeline |

## Smoke-тесты

| Тест | Что проверяется |
|---|---|
| TestHealthcheck | `/healthcheck` → 200 |
| TestHealth | `/health` → 200 |
| TestLive | `/live` → 200 |
| TestReady | `/ready` → 200 |

## Пакеты без unit-тестов

| Пакет | Причина |
|---|---|
| internal/service/facade | Тонкий proxy (11 методов), покрыт через transport/grpc тесты |
