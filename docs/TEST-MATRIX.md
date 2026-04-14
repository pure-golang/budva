# Матрица тестирования — budva-claude

## Контекст

Сервис пересылки/копирования сообщений между чатами Telegram. Принимает обновления (new/edit/delete), применяет правила (фильтрация, трансформация, дедупликация), пересылает в целевые чаты. Внешние интерфейсы: gRPC (FacadeGRPC), HTTP (REST auth + GraphQL), Terminal (CLI авторизация). Инфраструктура: BadgerDB (state), YAML (ruleset), in-memory queue.

**Архитектура:**
- `internal/handler` — диспетчер обновлений: OnNewMessage, OnEditedMessage, OnDeletedMessages
- `internal/service/*` — бизнес-логика: auth, transform, filters, dedup, album, limiter, message, facade
- `internal/repo/*` — адаптеры: state (BadgerDB), ruleset (YAML), queue, telegram (TDLib stub)
- `internal/transport/*` — транспорты: grpc, http (REST + GraphQL), term
- `internal/domain` — доменные типы и утилиты
- `internal/config` — envconfig
- `internal/controller` — health endpoints

**Структура тестов:**
- `internal/**/*_test.go` — юнит-тесты рядом с исходниками, `t.Parallel()`
- `test/integration/*_test.go` — интеграционные тесты, `testing.Short()` skip
- `test/bdd/steps/*_test.go` + `test/bdd/features/` — BDD-тесты через godog, `testing.Short()` skip
- `test/smoke/*_test.go` — смоук-тесты, build tag `smoke`

---

## Покрытие по пакетам

| Пакет | Unit | Integration | BDD | Smoke |
|-------|------|-------------|-----|-------|
| internal/config | ✅ 3 | — | — | — |
| internal/controller | ✅ 3 | — | — | ✅ |
| internal/domain | ✅ 13 | — | — | — |
| internal/handler | ✅ 12 | ✅ | ✅ | — |
| internal/repo/queue | ✅ 4 | — | — | — |
| internal/repo/ruleset | ✅ 6 | — | — | — |
| internal/repo/state | ✅ 11 | ✅ | — | — |
| internal/repo/telegram | ✅ 2 | — | — | — |
| internal/service/album | ✅ 10 | — | ✅ | — |
| internal/service/auth | ✅ 8 | — | — | — |
| internal/service/dedup | ✅ 3 | — | ✅ | — |
| internal/service/facade | — | — | — | — |
| internal/service/filters | ✅ 8 | ✅ | ✅ | — |
| internal/service/limiter | ✅ 1 | — | ✅ | — |
| internal/service/message | ✅ 7 | — | — | — |
| internal/service/transform | ✅ 16 | ✅ | ✅ | — |
| internal/transport/grpc | ✅ 18 | — | — | — |
| internal/transport/http | ✅ 12 | — | — | — |
| internal/transport/http/graph | ✅ 5 | — | — | — |
| internal/transport/term | ✅ 3 | — | — | — |

---

## Unit-тесты (internal/)

### Config (internal/config)

Файл: `config_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| CFG-U-001 | TestTelegramConfig_RequiredFields | ✅ |
| CFG-U-002 | TestStorageConfig_Defaults | ✅ |
| CFG-U-003 | TestRulesetConfig_Defaults | ✅ |

---

### Controller (internal/controller)

Файл: `controller_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| CTRL-U-001 | TestLive_always_200 | ✅ |
| CTRL-U-002 | TestHealthcheck_all_healthy | ✅ |
| CTRL-U-003 | TestHealthcheck_unhealthy | ✅ |

---

### Domain (internal/domain)

Файл: `phone_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| DOM-U-001 | TestMaskPhoneNumber (11 subtests) | ✅ |
| DOM-U-002 | TestMaskPhoneNumber_PreservesLength | ✅ |

---

### Handler (internal/handler)

Файл: `handler_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| HDL-U-001 | TestOnNewMessage_NoRuleSet | ✅ |
| HDL-U-002 | TestOnNewMessage_UnknownSource | ✅ |
| HDL-U-003 | TestOnNewMessage_SystemMessage_DeleteEnabled | ✅ |
| HDL-U-004 | TestOnNewMessage_ForwardWithoutCopy | ✅ |
| HDL-U-005 | TestOnNewMessage_SendCopy | ✅ |
| HDL-U-006 | TestOnNewMessage_FiltersCheck | ✅ |
| HDL-U-007 | TestOnNewMessage_CannotBeSaved_WithoutSendCopy | ✅ |
| HDL-U-008 | TestOnDeletedMessages_PermanentWithCopies | ✅ |
| HDL-U-009 | TestOnDeletedMessages_IndelibleRule | ✅ |
| HDL-U-010 | TestOnDeletedMessages_RetryOnMissingNewID | ✅ |
| HDL-U-011 | TestOnMessageSendSucceeded | ✅ |
| HDL-U-012 | TestSetRuleSet | ✅ |

---

### Repo / Queue (internal/repo/queue)

Файл: `repo_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| REPO-U-001 | TestRepo_Add_and_Len | ✅ |
| REPO-U-002 | TestRepo_ProcessQueue_executes_task | ✅ |
| REPO-U-003 | TestRepo_ProcessQueue_recovers_from_panic | ✅ |
| REPO-U-004 | TestRepo_StartContext_processes_tasks | ✅ |

---

### Repo / Ruleset (internal/repo/ruleset)

Файл: `repo_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| REPO-U-005 | TestRepo_Load | ✅ |
| REPO-U-006 | TestRepo_Load_empty_config | ✅ |
| REPO-U-007 | TestRepo_Load_file_not_found | ✅ |
| REPO-U-008 | TestRepo_Load_Negation | ✅ |
| REPO-U-009 | TestRepo_Load_FragmentUTF16Validation | ✅ |
| REPO-U-010 | TestRepo_Load_FullPipeline | ✅ |

---

### Repo / State (internal/repo/state)

Файлы: `repo_test.go`, `copies_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| REPO-U-011 | TestRepo_SetGet | ✅ |
| REPO-U-012 | TestRepo_Get_not_found | ✅ |
| REPO-U-013 | TestRepo_Delete | ✅ |
| REPO-U-014 | TestRepo_GetSet_atomic | ✅ |
| REPO-U-015 | TestSetCopiedMessageID_SingleDestination | ✅ |
| REPO-U-016 | TestSetCopiedMessageID_MultipleDestinations | ✅ |
| REPO-U-017 | TestSetCopiedMessageID_UpdateInPlace | ✅ |
| REPO-U-018 | TestDeleteCopiedMessageIDs | ✅ |
| REPO-U-019 | TestNewMessageID_Bidirectional | ✅ |
| REPO-U-020 | TestIncrementCounters | ✅ |
| REPO-U-021 | TestAnswerMessageID | ✅ |

---

### Repo / Telegram (internal/repo/telegram)

Файл: `auth_flow_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| REPO-U-022 | TestRunAuthFlow_FullCycle | ✅ |
| REPO-U-023 | TestRunAuthFlow_CancelDuringInput | ✅ |

---

### Service / Album (internal/service/album)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-001 | TestAddMessage_FirstReturnsTrue | ✅ |
| SVC-U-002 | TestAddMessage_SecondReturnsFalse | ✅ |
| SVC-U-003 | TestAddMessage_DifferentKeys | ✅ |
| SVC-U-004 | TestPopMessages | ✅ |
| SVC-U-005 | TestPopMessages_RemovesAlbum | ✅ |
| SVC-U-006 | TestPopMessages_EmptyKey | ✅ |
| SVC-U-007 | TestLastReceivedAge | ✅ |
| SVC-U-008 | TestLastReceivedAge_NonexistentKey | ✅ |
| SVC-U-009 | TestLastReceivedAge_UpdatedOnNewMessage | ✅ |
| SVC-U-010 | TestMakeKey | ✅ |

---

### Service / Auth (internal/service/auth)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-011 | TestNew | ✅ |
| SVC-U-012 | TestSetStateAndState | ✅ |
| SVC-U-013 | TestSubscribeReceivesStateChanges | ✅ |
| SVC-U-014 | TestSubscribeReceivesExtra | ✅ |
| SVC-U-015 | TestMultipleSubscribers | ✅ |
| SVC-U-016 | TestInputChanSend | ✅ |
| SVC-U-017 | TestReadInput | ✅ |
| SVC-U-018 | TestConcurrentStateAccess | ✅ |

---

### Service / Dedup (internal/service/dedup)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-019 | TestTracker_TryMark_first_time | ✅ |
| SVC-U-020 | TestTracker_TryMark_duplicate | ✅ |
| SVC-U-021 | TestTracker_TryMark_unknown_chat | ✅ |

---

### Service / Filters (internal/service/filters)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-022 | TestEvaluate_no_filters | ✅ |
| SVC-U-023 | TestEvaluate_exclude_matches | ✅ |
| SVC-U-024 | TestEvaluate_exclude_no_match | ✅ |
| SVC-U-025 | TestEvaluate_include_matches | ✅ |
| SVC-U-026 | TestEvaluate_include_no_match | ✅ |
| SVC-U-027 | TestEvaluate_empty_text_with_include | ✅ |
| SVC-U-028 | TestEvaluate_submatch | ✅ |
| SVC-U-029 | TestEvaluate_submatch_no_match | ✅ |

---

### Service / Limiter (internal/service/limiter)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-030 | TestWaitForForward | ✅ |

---

### Service / Message (internal/service/message)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-031 | TestGetFormattedText (5 subtests) | ✅ |
| SVC-U-032 | TestIsSystemMessage (3 subtests) | ✅ |
| SVC-U-033 | TestGetReplyMarkupData (3 subtests) | ✅ |
| SVC-U-034 | TestBuildInputContent_Photo | ✅ |
| SVC-U-035 | TestBuildInputContent_Text_InvertsLinkPreview | ✅ |
| SVC-U-036 | TestBuildInputContent_Document | ✅ |
| SVC-U-037 | TestBuildInputContent_VoiceNote | ✅ |

---

### Service / Transform (internal/service/transform)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-038 | TestTransform_NoTransformations | ✅ |
| SVC-U-039 | TestTransform_Translation | ✅ |
| SVC-U-040 | TestTransform_Translation_SkippedForOtherChat | ✅ |
| SVC-U-041 | TestTransform_ReplaceFragments | ✅ |
| SVC-U-042 | TestTransform_Sign | ✅ |
| SVC-U-043 | TestTransform_Link | ✅ |
| SVC-U-044 | TestTransform_PrevLink | ✅ |
| SVC-U-045 | TestAddNextLink | ✅ |
| SVC-U-046 | TestAddNextLink_NoNextConfig | ✅ |
| SVC-U-047 | TestAddNextLink_ChatNotInFor | ✅ |
| SVC-U-048 | TestEncodeDecodeUTF16 (4 subtests) | ✅ |
| SVC-U-049 | TestExtractSubstring | ✅ |
| SVC-U-050 | TestExtractSubstring_BeyondLength | ✅ |
| SVC-U-051 | TestReplaceFragment | ✅ |
| SVC-U-052 | TestReplaceFragment_NoMatch | ✅ |
| SVC-U-053 | TestReplaceFragment_NilText | ✅ |

---

### Transport / gRPC (internal/transport/grpc)

Файл: `transport_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| GRPC-U-001 | TestGetMessages_Success | ✅ |
| GRPC-U-002 | TestGetMessages_PartialFailure | ✅ |
| GRPC-U-003 | TestSendMessage_Success | ✅ |
| GRPC-U-004 | TestSendMessage_NilMessage | ✅ |
| GRPC-U-005 | TestSendMessage_FacadeError | ✅ |
| GRPC-U-006 | TestSendMessageAlbum_Success | ✅ |
| GRPC-U-007 | TestSendMessageAlbum_EmptyMessages | ✅ |
| GRPC-U-008 | TestForwardMessage_Success | ✅ |
| GRPC-U-009 | TestGetMessage_Success | ✅ |
| GRPC-U-010 | TestUpdateMessage_Success | ✅ |
| GRPC-U-011 | TestUpdateMessage_NilMessage | ✅ |
| GRPC-U-012 | TestDeleteMessages_Success | ✅ |
| GRPC-U-013 | TestGetMessageLink_Success | ✅ |
| GRPC-U-014 | TestGetMessageLinkInfo_Success | ✅ |
| GRPC-U-015 | TestGetChatHistory_Success | ✅ |
| GRPC-U-016 | TestGetChatHistory_Empty | ✅ |
| GRPC-U-017 | TestDomainToProto_Nil | ✅ |
| GRPC-U-018 | TestDomainToProto_WithForwardInfo | ✅ |

---

### Transport / HTTP (internal/transport/http)

Файл: `transport_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| HTTP-U-001 | TestGetState_WaitPhone | ✅ |
| HTTP-U-002 | TestGetState_Ready | ✅ |
| HTTP-U-003 | TestGetState_PasswordHint | ✅ |
| HTTP-U-004 | TestPostPhone_Success | ✅ |
| HTTP-U-005 | TestPostPhone_EmptyPhone | ✅ |
| HTTP-U-006 | TestPostPhone_InvalidJSON | ✅ |
| HTTP-U-007 | TestPostPhone_NoBody | ✅ |
| HTTP-U-008 | TestPostCode_Success | ✅ |
| HTTP-U-009 | TestPostCode_EmptyCode | ✅ |
| HTTP-U-010 | TestPostPassword_Success | ✅ |
| HTTP-U-011 | TestPostPassword_EmptyPassword | ✅ |
| HTTP-U-012 | TestResponseContentType | ✅ |

---

### Transport / HTTP / Graph (internal/transport/http/graph)

Файл: `handler_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| GQL-U-001 | TestHandler_StatusQuery | ✅ |
| GQL-U-002 | TestHandler_StatusError | ✅ |
| GQL-U-003 | TestHandler_InvalidBody | ✅ |
| GQL-U-004 | TestHandler_UnknownQuery | ✅ |
| GQL-U-005 | TestPlaygroundHandler | ✅ |

---

### Transport / Term (internal/transport/term)

Файл: `transport_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| TERM-U-001 | TestRunInputLoop_Exit | ✅ |
| TERM-U-002 | TestProcessAuth_WaitPassword/with_hint | ✅ |
| TERM-U-003 | TestProcessAuth_WaitPassword/without_hint | ✅ |

---

## Интеграционные тесты (test/integration/)

Требуют: BadgerDB (TempDir), FakeTelegram, support.Stack. Skip: `testing.Short()`.

### Forward pipeline (forward_pipeline_test.go)

| ID | Тест | Статус |
|----|------|--------|
| INT-001 | TestForwardPipeline/copy_with_transform | ✅ |
| INT-002 | TestForwardPipeline/edit_sync | ✅ |
| INT-003 | TestForwardPipeline/delete_sync | ✅ |
| INT-004 | TestForwardPipeline/filter_exclude | ✅ |

---

## BDD-тесты (test/bdd/)

Требуют: BadgerDB (TempDir), FakeTelegram, support.Stack. Skip: `testing.Short()`.

### 01_delivery

Steps: `01_delivery_steps_test.go`

| ID | Feature | Сценарий | Статус |
|----|---------|----------|--------|
| BDD-001 | 01_copy | 01. Копирование во все целевые чаты (×4) | ✅ |
| BDD-002 | 02_forward | 01. Пересылка во все целевые чаты (×4) | ✅ |
| BDD-003 | 03_rate_limiting | 01. Не чаще раза в 3 секунды | ✅ |
| BDD-004 | 04_reply_chain | 01. Ответ сохраняет связь | ✅ |
| BDD-005 | 05_origin_unwrapping | 01. Разворачивание до оригинала | ✅ |
| BDD-006 | 06_statistics | 01. Счётчики просмотренных/пересланных | ✅ |
| BDD-007 | 07_system_messages | 01. Удаление при включённом флаге | ✅ |
| BDD-008 | 07_system_messages | 02. Игнорирование при выключенном | ✅ |

### 02_filters

Steps: `02_filters_steps_test.go`

| ID | Feature | Сценарий | Статус |
|----|---------|----------|--------|
| BDD-009 | 01_exclude | 01. Без паттерна проходит (×8) | ✅ |
| BDD-010 | 01_exclude | 02. С паттерном блокируется (×8) | ✅ |
| BDD-011 | 02_include | 01. С разрешённым проходит (×8) | ✅ |
| BDD-012 | 03_submatch | 01. Submatch-фильтр (×8) | ✅ |
| BDD-013 | 04_check_other_dedup | 01. В check-чат один раз | ✅ |

### 03_transform

Steps: `03_transform_steps_test.go`

| ID | Feature | Сценарий | Статус |
|----|---------|----------|--------|
| BDD-014 | 01_replace_own_links | 01. Замена ссылок (×3) | ✅ |
| BDD-015 | 02_remove_external_links | 01. Удаление внешних (×3) | ✅ |
| BDD-016 | 03_replace_fragments | 01. Замена фрагментов (×4) | ✅ |
| BDD-017 | 04_source_link | 01. Ссылка на оригинал (×3) | ✅ |
| BDD-018 | 05_source_sign | 01. Подпись источника (×4) | ✅ |
| BDD-019 | 06_translate | 01. Перевод (×4) | ✅ |

### 04_media

Steps: `04_media_steps_test.go`

| ID | Feature | Сценарий | Статус |
|----|---------|----------|--------|
| BDD-020 | 01_album_copy | 01. Альбом копируется (×4) | ✅ |
| BDD-021 | 02_album_forward | 01. Альбом пересылается (×4) | ✅ |

### 05_sync

Steps: `05_sync_steps_test.go`

| ID | Feature | Сценарий | Статус |
|----|---------|----------|--------|
| BDD-022 | 01_versioning | 01. Новая версия со ссылками (×3) | ✅ |
| BDD-023 | 02_edit_update | 01. Обновление копии (×4) | ✅ |
| BDD-024 | 03_indelible | 01. Неудаляемые копии (×4) | ✅ |
| BDD-025 | 04_delete_sync | 01. Удаление копий (×4) | ✅ |
| BDD-026 | 05_retry | 01. Retry при отсутствии permanent ID | ✅ |

### 06_auto

Steps: `06_auto_steps_test.go`

| ID | Feature | Сценарий | Статус |
|----|---------|----------|--------|
| BDD-027 | 01_auto_answers | 01. Автоответ на callback (×3) | ✅ |

---

## Смоук-тесты (test/smoke/)

Требуют: Docker, testcontainers-compose, Dockerfile. Build tag: `smoke`.

Файл: `smoke_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SMOKE-001 | TestHealthcheck | ✅ |
| SMOKE-002 | TestHealth | ✅ |
| SMOKE-003 | TestLive | ✅ |
| SMOKE-004 | TestReady | ✅ |

---

## Сводная таблица

| Пакет | Unit | Integration | BDD | Smoke |
|-------|------|-------------|-----|-------|
| internal/config | 3 | — | — | — |
| internal/controller | 3 | — | — | 4 |
| internal/domain | 13 | — | — | — |
| internal/handler | 12 | 4 | 27 | — |
| internal/repo/queue | 4 | — | — | — |
| internal/repo/ruleset | 6 | — | — | — |
| internal/repo/state | 11 | 4 | — | — |
| internal/repo/telegram | 2 | — | — | — |
| internal/service/album | 10 | — | 8 | — |
| internal/service/auth | 8 | — | — | — |
| internal/service/dedup | 3 | — | 1 | — |
| internal/service/facade | — | — | — | — |
| internal/service/filters | 8 | 1 | 25 | — |
| internal/service/limiter | 1 | — | 1 | — |
| internal/service/message | 7 | — | — | — |
| internal/service/transform | 16 | 1 | 21 | — |
| internal/transport/grpc | 18 | — | — | — |
| internal/transport/http | 12 | — | — | — |
| internal/transport/http/graph | 5 | — | — | — |
| internal/transport/term | 3 | — | — | — |
| **Итого** | **133** | **4** | **27 scenarios** | **4** |

---

## Покрытие кода

Снято командой `task cover` — `-coverpkg=./...`, `-covermode=atomic`. Общее покрытие: **67.8%**.

| Пакет | Покрытие | Примечание |
|-------|----------|------------|
| internal/controller | 65.0% | health endpoints |
| internal/domain | 60.5% | MaskPhoneNumber |
| internal/handler | 72.4% | диспетчер обновлений |
| internal/repo/queue | 92.2% | in-memory queue |
| internal/repo/ruleset | 70.4% | YAML loader |
| internal/repo/state | 81.8% | BadgerDB CRUD + copies |
| internal/repo/telegram | 42.0% | auth flow stub |
| internal/repo/term | 0.0% | readline adapter, нет тестов |
| internal/service/album | 100.0% | — |
| internal/service/auth | 81.8% | pub-sub + state |
| internal/service/dedup | 100.0% | — |
| internal/service/facade | 0.0% | тонкий proxy, нет unit-тестов |
| internal/service/filters | 81.8% | evaluate + submatch |
| internal/service/limiter | 90.0% | WaitForForward |
| internal/service/message | 61.5% | GetFormattedText, BuildInputContent |
| internal/service/transform | 46.7% | transform pipeline, UTF-16 |
| internal/transport/grpc | 87.3% | все RPC |
| internal/transport/http | 80.8% | REST auth |
| internal/transport/http/graph | 96.0% | GraphQL handler |
| internal/transport/term | 51.8% | CLI auth + commands |
