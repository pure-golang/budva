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
| BDD-001 | `01_copy.feature` | `01. Сообщение копируется во все целевые чаты` | ×4 |
| BDD-002 | `01_copy.feature` | `02. Копирование из конкретного источника в конкретный целевой чат` | ×16 |
| BDD-003 | `02_forward.feature` | `01. Сообщение пересылается во все целевые чаты` | ×4 |
| BDD-004 | `02_forward.feature` | `02. Пересылка из конкретного источника в конкретный целевой чат` | ×16 |
| BDD-005 | `03_rate_limiting.feature` | `01. Пересылка в один чат не чаще раза в 3 секунды` | ×1 |
| BDD-006 | `04_reply_chain.feature` | `01. Ответ на сообщение сохраняет связь в целевом чате` | ×1 |
| BDD-007 | `05_origin_unwrapping.feature` | `01. Пересланное из канала сообщение разворачивается до оригинала` | ×1 |
| BDD-008 | `06_statistics.feature` | `01. Просмотренные и пересланные сообщения считаются` | ×1 |
| BDD-009 | `07_system_messages.feature` | `01. Системное сообщение удаляется при включённом флаге` | ×1 |
| BDD-010 | `07_system_messages.feature` | `02. Системное сообщение игнорируется при выключенном флаге` | ×1 |

Итого `01_delivery`: **46 scenario instances**.

### 02_filters

Файлы: `test/bdd/02_filters/*.feature`, `test/bdd/02_filters/bdd_test.go`, `test/bdd/shared/steps_02_filters.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-011 | `01_exclude.feature` | `01. Сообщение без запрещённого паттерна проходит фильтр` | ×8 |
| BDD-012 | `01_exclude.feature` | `02. Фильтр исключения из конкретного источника в конкретный целевой чат` | ×32 |
| BDD-013 | `01_exclude.feature` | `03. Сообщение с запрещённым паттерном блокируется` | ×8 |
| BDD-014 | `01_exclude.feature` | `04. Блокировка запрещённого паттерна из конкретного источника в конкретный целевой чат` | ×32 |
| BDD-015 | `02_include.feature` | `01. Сообщение с разрешённым паттерном проходит фильтр` | ×8 |
| BDD-016 | `02_include.feature` | `02. Фильтр включения из конкретного источника в конкретный целевой чат` | ×32 |
| BDD-017 | `03_submatch.feature` | `01. Сообщение с тикером проходит submatch-фильтр` | ×8 |
| BDD-018 | `03_submatch.feature` | `02. Submatch-фильтр из конкретного источника в конкретный целевой чат` | ×32 |
| BDD-019 | `04_check_other_dedup.feature` | `01. Сообщение отправляется в check-чат только один раз` | ×1 |

Итого `02_filters`: **161 scenario instances**.

### 03_transform

Файлы: `test/bdd/03_transform/*.feature`, `test/bdd/03_transform/bdd_test.go`, `test/bdd/shared/steps_03_transform.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-020 | `01_replace_own_links.feature` | `01. Ссылка на своё сообщение заменяется на ссылку в целевом чате` | ×3 |
| BDD-021 | `01_replace_own_links.feature` | `02. Замена ссылок из конкретного источника в конкретный целевой чат` | ×12 |
| BDD-022 | `02_remove_external_links.feature` | `01. Сообщение с внешней ссылкой копируется без неё` | ×3 |
| BDD-023 | `02_remove_external_links.feature` | `02. Удаление внешних ссылок из конкретного источника в конкретный целевой чат` | ×12 |
| BDD-024 | `03_replace_fragments.feature` | `01. Фрагменты текста заменяются по правилам` | ×4 |
| BDD-025 | `03_replace_fragments.feature` | `02. Замена фрагментов из конкретного источника в конкретный целевой чат` | ×16 |
| BDD-026 | `04_source_link.feature` | `01. К копии добавляется ссылка на оригинал` | ×3 |
| BDD-027 | `04_source_link.feature` | `02. Ссылка на оригинал из конкретного источника в конкретный целевой чат` | ×12 |
| BDD-028 | `05_source_sign.feature` | `01. К копии добавляется подпись источника` | ×4 |
| BDD-029 | `05_source_sign.feature` | `02. Подпись источника из конкретного источника в конкретный целевой чат` | ×16 |
| BDD-030 | `06_translate.feature` | `01. Сообщение копируется с переводом` | ×4 |
| BDD-031 | `06_translate.feature` | `02. Перевод из конкретного источника в конкретный целевой чат` | ×16 |

Итого `03_transform`: **105 scenario instances**.

### 04_media

Файлы: `test/bdd/04_media/*.feature`, `test/bdd/04_media/bdd_test.go`, `test/bdd/shared/steps_04_media.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-032 | `01_album_copy.feature` | `01. Медиа-альбом копируется как единое целое` | ×4 |
| BDD-033 | `01_album_copy.feature` | `02. Копирование альбома из конкретного источника в конкретный целевой чат` | ×16 |
| BDD-034 | `02_album_forward.feature` | `01. Медиа-альбом пересылается как единое целое` | ×4 |
| BDD-035 | `02_album_forward.feature` | `02. Пересылка альбома из конкретного источника в конкретный целевой чат` | ×16 |

Итого `04_media`: **40 scenario instances**.

### 05_sync

Файлы: `test/bdd/05_sync/*.feature`, `test/bdd/05_sync/bdd_test.go`, `test/bdd/shared/steps_05_sync.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-036 | `01_versioning.feature` | `01. Редактирование создаёт новую версию со ссылками` | ×3 |
| BDD-037 | `01_versioning.feature` | `02. Версионирование из конкретного источника в конкретный целевой чат` | ×12 |
| BDD-038 | `02_edit_update.feature` | `01. Редактирование обновляет существующую копию` | ×4 |
| BDD-039 | `02_edit_update.feature` | `02. Обновление копии из конкретного источника в конкретный целевой чат` | ×16 |
| BDD-040 | `03_indelible.feature` | `01. Удаление оригинала не удаляет копии` | ×4 |
| BDD-041 | `03_indelible.feature` | `02. Неудаляемые копии из конкретного источника в конкретный целевой чат` | ×16 |
| BDD-042 | `04_delete_sync.feature` | `01. Удаление оригинала удаляет все копии` | ×4 |
| BDD-043 | `04_delete_sync.feature` | `02. Синхронизация удаления из конкретного источника в конкретный целевой чат` | ×16 |
| BDD-044 | `05_retry_eventual_consistency.feature` | `01. Удаление повторяется если permanent ID ещё не в хранилище` | ×1 |

Итого `05_sync`: **76 scenario instances**.

### 06_auto

Файлы: `test/bdd/06_auto/*.feature`, `test/bdd/06_auto/bdd_test.go`, `test/bdd/shared/steps_06_auto.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-045 | `01_auto_answers.feature` | `01. Бот автоматически отвечает на callback-запрос` | ×3 |
| BDD-046 | `01_auto_answers.feature` | `02. Автоответ из конкретного источника в конкретный целевой чат` | ×12 |

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
