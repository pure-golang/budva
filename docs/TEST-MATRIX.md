# Матрица тестирования — budva-claude

## Контекст

Сервис пересылки и копирования сообщений между чатами Telegram. Поток обновлений обрабатывается через `internal/handler`, правила загружаются из YAML (`internal/repo/ruleset`), состояние и связи копий хранятся в BadgerDB (`internal/repo/state`), отправка и авторизация идут через TDLib (`internal/repo/telegram`). Внешние интерфейсы: gRPC facade, HTTP auth/GraphQL, terminal transport.

**Архитектура:**
- `internal/handler` — оркестрация `new/edit/delete` update и маршрутизация по ruleset
- `internal/service/*` — бизнес-логика: auth, facade, transform, filters, limiter, album, dedup, message
- `internal/repo/*` — адаптеры инфраструктуры: queue, ruleset, state, telegram, term
- `internal/transport/*` — gRPC, HTTP/GraphQL и terminal-транспорты
- `internal/controller` — health/readiness/live endpoints
- `internal/domain`, `internal/config` — доменные типы и конфигурация

**Структура тестов:**
- `internal/**/*_test.go` — unit-тесты рядом с кодом, быстрая локальная петля через `task test-short`
- `test/bdd/NN_epic/*.feature` + `test/bdd/NN_epic/bdd_test.go` — BDD через godog и LiveStack, запускаются в полном `task test` / `task cover`
- `test/smoke/smoke_test.go` — smoke-слой под build tag `smoke`

**Снятие покрытия:**
- `task cover` успешно выполнен `2026-04-21`
- общий coverage по production-коду после фильтрации `.coverage/.txt`: **98.0%**

---

## Покрытие по пакетам

| Пакет | Unit | BDD | Smoke |
|-------|------|-----|-------|
| `internal/config` | 4 | — | — |
| `internal/controller` | 5 | — | 4 |
| `internal/domain` | 3 | — | — |
| `internal/handler` | 75 | 443 scenario instances | — |
| `internal/repo/queue` | 14 | — | — |
| `internal/repo/ruleset` | 21 | — | — |
| `internal/repo/state` | 25 | — | — |
| `internal/repo/telegram` | 23 | 443 scenario instances | — |
| `internal/repo/term` | 6 | — | — |
| `internal/service/album` | 22 | 40 scenario instances | — |
| `internal/service/auth` | 20 | — | — |
| `internal/service/dedup` | 10 | 1 scenario instance | — |
| `internal/service/facade` | 14 | — | — |
| `internal/service/filters` | 8 | 161 scenario instances | — |
| `internal/service/limiter` | 14 | 1 scenario instance | — |
| `internal/service/message` | 40 | — | — |
| `internal/service/transform` | 51 | 105 scenario instances | — |
| `internal/transport/grpc` | 37 | — | — |
| `internal/transport/http` | 14 | — | — |
| `internal/transport/http/resolvers` | 2 | — | — |
| `internal/transport/term` | 23 | — | — |

---

## Unit-тесты (internal/)

### Config (`internal/config`)

Файлы: `config_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| CFG-U-001 | `config_test.go` | 4 tests: required/defaults для `TelegramConfig`, `StorageConfig`, `RulesetConfig` | ✅ |

---

### Controller (`internal/controller`)

Файлы: `controller_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| CTRL-U-001 | `controller_test.go` | 5 tests: `/live`, `/healthcheck`, `/ready` в healthy/unhealthy ветках | ✅ |

---

### Domain (`internal/domain`)

Файлы: `domain_test.go`, `phone_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| DOM-U-001 | `domain_test.go` | 1 test: строковое представление `AuthorizationState` | ✅ |
| DOM-U-002 | `phone_test.go` | 2 tests: mask phone и сохранение длины | ✅ |

---

### Handler (`internal/handler`)

Файлы: `handler_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| HDL-U-001 | `handler_test.go` | 75 tests: полный цикл `OnNewMessage`, `OnEditedMessage`, `OnDeletedMessages`, `runNextLinkWorkflow`, parse helpers, media album, dedup, reply-chain | ✅ |

---

### Repo / Queue (`internal/repo/queue`)

Файлы: `repo_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| REPO-U-001 | `repo_test.go` | 14 tests: `Add/Len`, `ProcessQueue`, `ProcessAll`, `ProcessBatch`, shutdown и edge-cases пустой очереди | ✅ |

---

### Repo / Ruleset (`internal/repo/ruleset`)

Файлы: `repo_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| REPO-U-002 | `repo_test.go` | 21 tests: `Load`, YAML validation, negate/enrich, watcher lifecycle, invalid path/YAML, UTF-16 validation | ✅ |

---

### Repo / State (`internal/repo/state`)

Файлы: `repo_test.go`, `copies_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| REPO-U-003 | `repo_test.go` | 14 tests: CRUD, `GetSet`, `Ping`, GC, logger, error helpers, startup edge-cases | ✅ |
| REPO-U-004 | `copies_test.go` | 11 tests: copy mappings, tmp/new IDs, counters, invalid stored values, answer IDs | ✅ |

---

### Repo / Telegram (`internal/repo/telegram`)

Файлы: `repo_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| REPO-U-005 | `repo_test.go` | 23 tests: flood wait parsing, auth state mapping, `Submit*`, pending sends, `SendMessageAndWait`, update dispatch | ✅ |

---

### Repo / Term (`internal/repo/term`)

Файлы: `repo_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| REPO-U-006 | `repo_test.go` | 6 tests: `ReadLine`, `Printf/Println`, password reading, constructor | ✅ |

---

### Service / Album (`internal/service/album`)

Файлы: `service_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| SVC-U-001 | `service_test.go` | 22 tests: add/pop, ordering, key reuse, age tracking, concurrent operations, `MakeKey` | ✅ |

---

### Service / Auth (`internal/service/auth`)

Файлы: `service_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| SVC-U-002 | `service_test.go` | 20 tests: auth flow, subscribers, special-state skip, extra state, logout, cancel paths, concurrent subscribe/state access | ✅ |

---

### Service / Dedup (`internal/service/dedup`)

Файлы: `service_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| SVC-U-003 | `service_test.go` | 10 tests: tracker initialization, duplicate detection, table-driven and concurrent paths | ✅ |

---

### Service / Facade (`internal/service/facade`)

Файлы: `service_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| SVC-U-004 | `service_test.go` | 14 tests: facade proxy methods, album/file routing, status, build version helper | ✅ |

---

### Service / Filters (`internal/service/filters`)

Файлы: `service_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| SVC-U-005 | `service_test.go` | 8 tests: include/exclude/submatch, invalid regexp panic paths, large text, concurrent safety | ✅ |

---

### Service / Limiter (`internal/service/limiter`)

Файлы: `service_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| SVC-U-006 | `service_test.go` | 14 tests: per-chat interval limiting, cancellation, serialization and concurrency behaviour | ✅ |

---

### Service / Message (`internal/service/message`)

Файлы: `service_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| SVC-U-007 | `service_test.go` | 40 tests: `GetFormattedText`, `IsSystemMessage`, reply markup, `BuildInputContent` для text/media и nil-edge cases | ✅ |

---

### Service / Transform (`internal/service/transform`)

Файлы: `service_test.go`, `helpers_test.go`, `utf16_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| SVC-U-008 | `service_test.go` | 34 tests: translation, replace fragments, source link/sign, auto-answer, self-link replacement, next-link, `addText` | ✅ |
| SVC-U-009 | `helpers_test.go` | 13 tests: deep copy, entity URL helpers, fragment replacement, substring extraction, entity shifts | ✅ |
| SVC-U-010 | `utf16_test.go` | 4 tests: UTF-16 roundtrip, length, clamp and lone surrogate handling | ✅ |

---

### Transport / gRPC (`internal/transport/grpc`)

Файлы: `transport_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| GRPC-U-001 | `transport_test.go` | 37 tests: RPC wrappers, success/error paths, nil handling, proto conversion for supported TDLib content | ✅ |

---

### Transport / HTTP (`internal/transport/http`)

Файлы: `transport_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| HTTP-U-001 | `transport_test.go` | 14 tests: auth REST handlers, grouped validation cases, password hint, route enrichment, encode error branches | ✅ |

---

### Transport / HTTP / Resolvers (`internal/transport/http/resolvers`)

Файлы: `schema.resolvers_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| GQL-U-001 | `schema.resolvers_test.go` | 2 tests: `Status` success/error | ✅ |

---

### Transport / Term (`internal/transport/term`)

Файлы: `transport_test.go`

| ID | Тестовый файл | Покрытие | Статус |
|----|---------------|----------|--------|
| TERM-U-001 | `transport_test.go` | 23 tests: input loop, auth states, commands, logout/exit, run/close, read errors, status printing | ✅ |

---

## BDD-тесты (`test/bdd/`)

Требуют: авторизованную TDLib-сессию, тестовые чаты и LiveStack из `internal/test/support`. Внутри полного `task cover` epics запускаются как отдельные пакеты через `shared.RunEpic`.

### 01_delivery

Файлы: `test/bdd/01_delivery/*.feature`, `test/bdd/01_delivery/bdd_test.go`, `test/bdd/shared/steps_01_delivery.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-001 | `01_copy.feature` | `01_message_is_copied_to_all_target_chats` | ×4 |
| BDD-002 | `01_copy.feature` | `02_copy_from_specific_source_to_specific_target_chat` | ×16 |
| BDD-003 | `02_forward.feature` | `01_message_is_forwarded_to_all_target_chats` | ×4 |
| BDD-004 | `02_forward.feature` | `02_forward_from_specific_source_to_specific_target_chat` | ×16 |
| BDD-005 | `03_rate_limiting.feature` | `01_forwarding_to_one_chat_is_limited_to_once_every_3_seconds` | ×1 |
| BDD-006 | `04_reply_chain.feature` | `01_reply_to_message_keeps_relation_in_target_chat` | ×1 |
| BDD-007 | `05_origin_unwrapping.feature` | `01_forwarded_channel_message_is_unwrapped_to_original` | ×1 |
| BDD-008 | `06_statistics.feature` | `01_viewed_and_forwarded_messages_are_counted` | ×1 |
| BDD-009 | `07_system_messages.feature` | `01_system_message_is_deleted_when_flag_is_enabled` | ×1 |
| BDD-010 | `07_system_messages.feature` | `02_system_message_is_ignored_when_flag_is_disabled` | ×1 |

Итого `01_delivery`: **46 scenario instances**.

### 02_filters

Файлы: `test/bdd/02_filters/*.feature`, `test/bdd/02_filters/bdd_test.go`, `test/bdd/shared/steps_02_filters.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-011 | `01_exclude.feature` | `01_message_without_blocked_pattern_passes_filter` | ×8 |
| BDD-012 | `01_exclude.feature` | `02_exclude_filter_from_specific_source_to_specific_target_chat` | ×32 |
| BDD-013 | `01_exclude.feature` | `03_message_with_blocked_pattern_is_blocked` | ×8 |
| BDD-014 | `01_exclude.feature` | `04_message_with_blocked_pattern_is_blocked_from_specific_source_to_specific_target_chat` | ×32 |
| BDD-015 | `02_include.feature` | `01_message_with_allowed_pattern_passes_filter` | ×8 |
| BDD-016 | `02_include.feature` | `02_include_filter_from_specific_source_to_specific_target_chat` | ×32 |
| BDD-017 | `03_submatch.feature` | `01_message_with_ticker_passes_submatch_filter` | ×8 |
| BDD-018 | `03_submatch.feature` | `02_submatch_filter_from_specific_source_to_specific_target_chat` | ×32 |
| BDD-019 | `04_check_other_dedup.feature` | `01_message_is_sent_to_check_chat_only_once` | ×1 |

Итого `02_filters`: **161 scenario instances**.

### 03_transform

Файлы: `test/bdd/03_transform/*.feature`, `test/bdd/03_transform/bdd_test.go`, `test/bdd/shared/steps_03_transform.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-020 | `01_replace_own_links.feature` | `01_own_message_link_is_replaced_with_link_in_target_chat` | ×3 |
| BDD-021 | `01_replace_own_links.feature` | `02_replace_links_from_specific_source_to_specific_target_chat` | ×12 |
| BDD-022 | `02_remove_external_links.feature` | `01_message_with_external_link_is_copied_without_it` | ×3 |
| BDD-023 | `02_remove_external_links.feature` | `02_remove_external_links_from_specific_source_to_specific_target_chat` | ×12 |
| BDD-024 | `03_replace_fragments.feature` | `01_text_fragments_are_replaced_by_rules` | ×4 |
| BDD-025 | `03_replace_fragments.feature` | `02_replace_fragments_from_specific_source_to_specific_target_chat` | ×16 |
| BDD-026 | `04_source_link.feature` | `01_link_to_original_is_added_to_copy` | ×3 |
| BDD-027 | `04_source_link.feature` | `02_link_to_original_from_specific_source_to_specific_target_chat` | ×12 |
| BDD-028 | `05_source_sign.feature` | `01_source_signature_is_added_to_copy` | ×4 |
| BDD-029 | `05_source_sign.feature` | `02_source_signature_from_specific_source_to_specific_target_chat` | ×16 |
| BDD-030 | `06_translate.feature` | `01_message_is_copied_with_translation` | ×4 |
| BDD-031 | `06_translate.feature` | `02_translation_from_specific_source_to_specific_target_chat` | ×16 |

Итого `03_transform`: **105 scenario instances**.

### 04_media

Файлы: `test/bdd/04_media/*.feature`, `test/bdd/04_media/bdd_test.go`, `test/bdd/shared/steps_04_media.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-032 | `01_album_copy.feature` | `01_media_album_is_copied_as_single_unit` | ×4 |
| BDD-033 | `01_album_copy.feature` | `02_copy_album_from_specific_source_to_specific_target_chat` | ×16 |
| BDD-034 | `02_album_forward.feature` | `01_media_album_is_forwarded_as_single_unit` | ×4 |
| BDD-035 | `02_album_forward.feature` | `02_forward_album_from_specific_source_to_specific_target_chat` | ×16 |

Итого `04_media`: **40 scenario instances**.

### 05_sync

Файлы: `test/bdd/05_sync/*.feature`, `test/bdd/05_sync/bdd_test.go`, `test/bdd/shared/steps_05_sync.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-036 | `01_versioning.feature` | `01_editing_creates_new_version_with_links` | ×3 |
| BDD-037 | `01_versioning.feature` | `02_versioning_from_specific_source_to_specific_target_chat` | ×12 |
| BDD-038 | `02_edit_update.feature` | `01_editing_updates_existing_copy` | ×4 |
| BDD-039 | `02_edit_update.feature` | `02_update_copy_from_specific_source_to_specific_target_chat` | ×16 |
| BDD-040 | `03_indelible.feature` | `01_deleting_original_does_not_delete_copies` | ×4 |
| BDD-041 | `03_indelible.feature` | `02_indelible_copies_from_specific_source_to_specific_target_chat` | ×16 |
| BDD-042 | `04_delete_sync.feature` | `01_deleting_original_deletes_all_copies` | ×4 |
| BDD-043 | `04_delete_sync.feature` | `02_deletion_sync_from_specific_source_to_specific_target_chat` | ×16 |
| BDD-044 | `05_retry_eventual_consistency.feature` | `01_deletion_is_retried_if_permanent_id_is_missing_in_storage` | ×1 |

Итого `05_sync`: **76 scenario instances**.

### 06_auto

Файлы: `test/bdd/06_auto/*.feature`, `test/bdd/06_auto/bdd_test.go`, `test/bdd/shared/steps_06_auto.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-045 | `01_auto_answers.feature` | `01_bot_automatically_answers_callback_request` | ×3 |
| BDD-046 | `01_auto_answers.feature` | `02_auto_answer_from_specific_source_to_specific_target_chat` | ×12 |

Итого `06_auto`: **15 scenario instances**.

---

## Смоук-тесты (`test/smoke/`)

Требуют: Docker, `testcontainers-go/modules/compose`, образы `engine` и `facade`. Build tag: `smoke`.

Файл: `smoke_test.go`

| ID | Тест | Статус |
|----|------|--------|
| SMOKE-001 | `SmokeSuite.TestHealthcheck` | ✅ |
| SMOKE-002 | `SmokeSuite.TestHealth` | ✅ |
| SMOKE-003 | `SmokeSuite.TestLive` | ✅ |
| SMOKE-004 | `SmokeSuite.TestReady` | ✅ |

---

## Сводная таблица

| Пакет | Unit | BDD | Smoke |
|-------|------|-----|-------|
| `internal/config` | 4 | — | — |
| `internal/controller` | 5 | — | 4 |
| `internal/domain` | 3 | — | — |
| `internal/handler` | 75 | 443 | — |
| `internal/repo/queue` | 14 | — | — |
| `internal/repo/ruleset` | 21 | — | — |
| `internal/repo/state` | 25 | — | — |
| `internal/repo/telegram` | 23 | 443 | — |
| `internal/repo/term` | 6 | — | — |
| `internal/service/album` | 22 | 40 | — |
| `internal/service/auth` | 20 | — | — |
| `internal/service/dedup` | 10 | 1 | — |
| `internal/service/facade` | 14 | — | — |
| `internal/service/filters` | 8 | 161 | — |
| `internal/service/limiter` | 14 | 1 | — |
| `internal/service/message` | 40 | — | — |
| `internal/service/transform` | 51 | 105 | — |
| `internal/transport/grpc` | 37 | — | — |
| `internal/transport/http` | 14 | — | — |
| `internal/transport/http/resolvers` | 2 | — | — |
| `internal/transport/term` | 23 | — | — |
| **Итого** | **431** | **443** | **4** |

---

## Покрытие кода

Снято командой `task cover`. Общее покрытие: **98.0%**.

| Пакет | Покрытие | Примечание |
|-------|----------|------------|
| `internal/controller` | 100.0% | health endpoints |
| `internal/domain` | 100.0% | domain helpers и string mapping |
| `internal/handler` | 100.0% | update dispatcher и orchestration |
| `internal/repo/queue` | 100.0% | in-memory queue |
| `internal/repo/ruleset` | 96.0% | YAML loader, validation, watcher |
| `internal/repo/state` | 95.9% | Badger CRUD, counters, copy mappings |
| `internal/repo/telegram` | 88.6% | TDLib wrapper, auth mapping, send/wait, update dispatch |
| `internal/repo/term` | 91.7% | terminal adapter |
| `internal/service/album` | 100.0% | album grouping |
| `internal/service/auth` | 100.0% | auth flow orchestration |
| `internal/service/dedup` | 100.0% | dedup tracker |
| `internal/service/facade` | 98.1% | facade proxy logic |
| `internal/service/filters` | 100.0% | include/exclude/submatch |
| `internal/service/limiter` | 100.0% | per-chat rate limiter |
| `internal/service/message` | 100.0% | formatted text, reply markup, input content builders |
| `internal/service/transform` | 99.4% | transform pipeline, link replacement, UTF-16 helpers |
| `internal/transport/grpc` | 100.0% | gRPC wrappers и proto mapping |
| `internal/transport/http` | 100.0% | REST auth handlers |
| `internal/transport/http/resolvers` | 100.0% | GraphQL resolver |
| `internal/transport/term` | 100.0% | CLI auth and command loop |
