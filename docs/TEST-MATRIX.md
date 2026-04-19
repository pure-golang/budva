# Матрица тестирования — budva-claude

## Контекст

Сервис пересылки/копирования сообщений между чатами Telegram. Принимает обновления (new/edit/delete), применяет правила (фильтрация, трансформация, дедупликация), пересылает в целевые чаты. Внешние интерфейсы: gRPC (FacadeGRPC), HTTP (REST auth + GraphQL), Terminal (CLI авторизация). Инфраструктура: BadgerDB (state), YAML (ruleset), in-memory queue, TDLib (Telegram client).

**Архитектура:**
- `internal/handler` — диспетчер обновлений: OnNewMessage, OnEditedMessage, OnDeletedMessages
- `internal/service/*` — бизнес-логика: auth, transform, filters, dedup, album, limiter, message, facade
- `internal/repo/*` — адаптеры: state (BadgerDB), ruleset (YAML), queue, telegram (TDLib через clientAdapter + mapping)
- `internal/transport/*` — транспорты: grpc, http (REST + GraphQL), term
- `internal/domain` — доменные типы и утилиты
- `internal/config` — envconfig (включая TelegramConfig с полной параметризацией TDLib)
- `internal/controller` — health endpoints

**Структура тестов:**
- `internal/**/*_test.go` — юнит-тесты рядом с исходниками, `t.Parallel()`
- `test/bdd/NN_epic/bdd_test.go` + `test/bdd/NN_epic/*.feature` — BDD-тесты через godog, `testing.Short()` skip
- `test/smoke/*_test.go` — смоук-тесты, build tag `smoke`

**Тестовая инфраструктура BDD:**
- `internal/test/support/live_stack.go` — LiveStack: полный стек с реальным TDLib и тестовыми чатами из фикстур
- `internal/test/support/fixtures.go` — загрузка фикстур тестовых чатов из JSON
- `test/bdd/shared/` — общий пакет: ScenarioCtx, RunEpic, flock, prefix, steps per epic

---

## Покрытие по пакетам

| Пакет | Unit | BDD | Smoke |
|-------|------|-----|-------|
| internal/config | ✅ 4 | — | — |
| internal/controller | ✅ 5 | — | ✅ |
| internal/domain | ✅ 7 | — | — |
| internal/handler | ✅ 20 | ✅ | — |
| internal/repo/queue | ✅ 4 | — | — |
| internal/repo/ruleset | ✅ 11 | — | — |
| internal/repo/state | ✅ 11 | — | — |
| internal/repo/telegram | ✅ 23 | ✅ | — |
| internal/repo/term | ✅ 6 | — | — |
| internal/service/album | ✅ 10 | ✅ | — |
| internal/service/auth | ✅ 21 | — | — |
| internal/service/dedup | ✅ 10 | ✅ | — |
| internal/service/facade | ✅ 14 | — | — |
| internal/service/filters | ✅ 8 | ✅ | — |
| internal/service/limiter | ✅ 14 | ✅ | — |
| internal/service/message | ✅ 10 | — | — |
| internal/service/transform | ✅ 16 | ✅ | — |
| internal/transport/grpc | ✅ 18 | — | — |
| internal/transport/http | ✅ 12 | — | — |
| internal/transport/http/graph | ✅ 5 | — | — |
| internal/transport/term | ✅ 3 | — | — |

---

## Unit-тесты (internal/)

### Config (internal/config)

Файл: `config_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| CFG-U-001 | TestTelegramConfig_RequiredFields | ✅ |
| CFG-U-002 | TestTelegramConfig_Defaults | ✅ |
| CFG-U-003 | TestStorageConfig_Defaults | ✅ |
| CFG-U-004 | TestRulesetConfig_Defaults | ✅ |

---

### Controller (internal/controller)

Файл: `controller_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| CTRL-U-001 | TestLive_always_200 | ✅ |
| CTRL-U-002 | TestHealthcheck_all_healthy | ✅ |
| CTRL-U-003 | TestHealthcheck_unhealthy | ✅ |
| CTRL-U-004 | TestReady_all_healthy | ✅ |
| CTRL-U-005 | TestReady_unhealthy | ✅ |

---

### Domain (internal/domain)

Файлы: `phone_test.go`, `domain_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| DOM-U-001 | TestMaskPhoneNumber (11 subtests) | ✅ |
| DOM-U-002 | TestMaskPhoneNumber_PreservesLength | ✅ |
| DOM-U-003 | TestAuthorizationState_String (7 subtests) | ✅ |
| DOM-U-004 | TestFormattedText_DeepCopy (3 subtests) | ✅ |
| DOM-U-005 | TestContentTypeByFileExt (20 subtests) | ✅ |
| DOM-U-006 | TestMessageContentType_IsMediaType (9 subtests) | ✅ |
| DOM-U-007 | TestMessageContentType_HasCaption (2 subtests) | ✅ |

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
| HDL-U-007 | TestOnNewMessage_FiltersOther | ✅ |
| HDL-U-008 | TestOnNewMessage_CannotBeSaved_WithoutSendCopy | ✅ |
| HDL-U-009 | TestOnEditedMessage_NoRuleSet | ✅ |
| HDL-U-010 | TestOnEditedMessage_UnknownSource | ✅ |
| HDL-U-011 | TestOnEditedMessage_TextUpdate | ✅ |
| HDL-U-012 | TestOnEditedMessage_CaptionUpdate | ✅ |
| HDL-U-013 | TestOnEditedMessage_CopyOnce_Versioning | ✅ |
| HDL-U-014 | TestOnEditedMessage_RetryOnMissingNewID | ✅ |
| HDL-U-015 | TestOnEditedMessage_ReplyMarkupSync | ✅ |
| HDL-U-016 | TestOnDeletedMessages_PermanentWithCopies | ✅ |
| HDL-U-017 | TestOnDeletedMessages_IndelibleRule | ✅ |
| HDL-U-018 | TestOnDeletedMessages_RetryOnMissingNewID | ✅ |
| HDL-U-019 | TestOnMessageSendSucceeded | ✅ |
| HDL-U-020 | TestSetRuleSet | ✅ |

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
| REPO-U-011 | TestRepo_Load_InvalidRuleID_Colon | ✅ |
| REPO-U-012 | TestRepo_Load_InvalidRuleID_Comma | ✅ |
| REPO-U-013 | TestRepo_Load_NegativeFrom | ✅ |
| REPO-U-014 | TestRepo_Load_NegativeTo | ✅ |
| REPO-U-015 | TestRepo_Load_FromEqualsTo | ✅ |

---

### Repo / State (internal/repo/state)

Файлы: `repo_test.go`, `copies_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| REPO-U-016 | TestRepo_SetGet | ✅ |
| REPO-U-017 | TestRepo_Get_not_found | ✅ |
| REPO-U-018 | TestRepo_Delete | ✅ |
| REPO-U-019 | TestRepo_GetSet_atomic | ✅ |
| REPO-U-020 | TestSetCopiedMessageID_SingleDestination | ✅ |
| REPO-U-021 | TestSetCopiedMessageID_MultipleDestinations | ✅ |
| REPO-U-022 | TestSetCopiedMessageID_UpdateInPlace | ✅ |
| REPO-U-023 | TestDeleteCopiedMessageIDs | ✅ |
| REPO-U-024 | TestNewMessageID_Bidirectional | ✅ |
| REPO-U-025 | TestIncrementCounters | ✅ |
| REPO-U-026 | TestAnswerMessageID | ✅ |

---

### Repo / Telegram (internal/repo/telegram)

Файл: `repo_internal_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| REPO-U-027 | TestParseFloodWait | ✅ |
| REPO-U-028 | TestIsRelevantUpdate | ✅ |
| REPO-U-029 | TestMapTDLibState | ✅ |
| REPO-U-030 | TestNew_InitializesChannels | ✅ |
| REPO-U-031 | TestClose_ResetsClientAdapter | ✅ |
| REPO-U-032 | TestSubmitPhone_WritesToChannel | ✅ |
| REPO-U-033 | TestSubmitCode_WritesToChannel | ✅ |
| REPO-U-034 | TestSubmitPassword_WritesToChannel | ✅ |
| REPO-U-035 | TestCleanUp | ✅ |
| REPO-U-036 | TestDispatchSendResult_SucceededDelivered | ✅ |
| REPO-U-037 | TestDispatchSendResult_FailedDelivered | ✅ |
| REPO-U-038 | TestDispatchSendResult_FailedWithNilError | ✅ |
| REPO-U-039 | TestDispatchSendResult_NoSubscriberIsNoOp | ✅ |
| REPO-U-040 | TestDispatchSendResult_IgnoresUnrelatedUpdates | ✅ |
| REPO-U-041 | TestPendingSends_ConcurrentAddRemove | ✅ |
| REPO-U-042 | TestSendMessageAndWait_SuccessPath | ✅ |
| REPO-U-043 | TestSendMessageAndWait_SendMessageError | ✅ |
| REPO-U-044 | TestSendMessageAndWait_ContextCancelled | ✅ |
| REPO-U-045 | TestSendMessageAndWait_FloodWaitExhaustsRetries | ✅ |
| REPO-U-046 | TestSendMessageAndWait_FloodWaitInterruptedByContext | ✅ |
| REPO-U-047 | TestSendMessageAndWait_DeliversErrorThroughPendingChannel | ✅ |
| REPO-U-048 | TestSendMessageAndWaitOnce_FloodWaitReturnedFromSend | ✅ |
| REPO-U-049 | TestSendMessageAndWaitOnce_NonFloodErrorReturnsZeroWait | ✅ |

Дополнительное покрытие через BDD-тесты (LiveStack с реальным TDLib).

---

### Repo / Term (internal/repo/term)

Файл: `repo_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| REPO-U-050 | TestNew | ✅ |
| REPO-U-051 | TestRepo_ReadLine | ✅ |
| REPO-U-052 | TestRepo_ReadLine_SequentialCalls | ✅ |
| REPO-U-053 | TestRepo_Println | ✅ |
| REPO-U-054 | TestRepo_Printf | ✅ |
| REPO-U-055 | TestRepo_ReadPassword_InvalidFD | ✅ |

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
| SVC-U-012 | TestStateUpdatedFromEvent | ✅ |
| SVC-U-013 | TestSubscribeReceivesStateChanges | ✅ |
| SVC-U-014 | TestSubscribeReceivesExtra | ✅ |
| SVC-U-015 | TestFullAuthFlow | ✅ |
| SVC-U-016 | TestMultipleSubscribers | ✅ |
| SVC-U-017 | TestCancelDuringWait | ✅ |
| SVC-U-018 | TestClosingStateIsSkipped | ✅ |
| SVC-U-019 | TestSubmitCodeRejection_WaitsForReEmit | ✅ |
| SVC-U-020 | TestFlowWithout2FA | ✅ |
| SVC-U-021 | TestClose_ClosesInputChan | ✅ |
| SVC-U-069 | TestExtra_InitiallyNil | ✅ |
| SVC-U-070 | TestExtra_ReturnsLastStateExtra | ✅ |
| SVC-U-071 | TestLogOut_Success | ✅ |
| SVC-U-072 | TestLogOut_RepoError_SkipsCleanUp | ✅ |
| SVC-U-073 | TestClosedStateIsSkipped | ✅ |
| SVC-U-074 | TestCancelBeforeEvent | ✅ |
| SVC-U-075 | TestSubmitPhoneError_Logged | ✅ |
| SVC-U-076 | TestSubmitPasswordError_Logged | ✅ |
| SVC-U-077 | TestConcurrentSubscribe | ✅ |
| SVC-U-078 | TestConcurrentStateReadWrite | ✅ |

---

### Service / Dedup (internal/service/dedup)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-022 | TestTracker_TryMark_first_time | ✅ |
| SVC-U-023 | TestTracker_TryMark_duplicate | ✅ |
| SVC-U-024 | TestTracker_TryMark_unknown_chat | ✅ |
| SVC-U-079 | TestNewTracker_empty_destinations | ✅ |
| SVC-U-080 | TestNewTracker_nil_slice | ✅ |
| SVC-U-081 | TestTracker_TryMark_independent_destinations | ✅ |
| SVC-U-082 | TestTracker_TryMark_table | ✅ |
| SVC-U-083 | TestTracker_TryMark_concurrent_same_chat | ✅ |
| SVC-U-084 | TestTracker_TryMark_concurrent_different_chats | ✅ |
| SVC-U-085 | TestTracker_independent_instances | ✅ |

---

### Service / Facade (internal/service/facade)

Файлы: `service_test.go`, `service_internal_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-025 | TestNew | ✅ |
| SVC-U-026 | TestService_GetMessage | ✅ |
| SVC-U-027 | TestService_SendMessage | ✅ |
| SVC-U-028 | TestService_SendMessageAlbum | ✅ |
| SVC-U-029 | TestService_ForwardMessage | ✅ |
| SVC-U-030 | TestService_UpdateMessage | ✅ |
| SVC-U-031 | TestService_DeleteMessages | ✅ |
| SVC-U-032 | TestService_GetChatHistory | ✅ |
| SVC-U-033 | TestService_GetMessages | ✅ |
| SVC-U-086 | TestService_GetMessageLink | ✅ |
| SVC-U-087 | TestService_GetMessageLinkInfo | ✅ |
| SVC-U-088 | TestService_GetStatus | ✅ |
| SVC-U-089 | TestInputMessageByFileExt | ✅ |
| SVC-U-090 | TestReleaseVersion | ✅ |

---

### Service / Filters (internal/service/filters)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-034 | TestEvaluate_no_filters | ✅ |
| SVC-U-035 | TestEvaluate_exclude_matches | ✅ |
| SVC-U-036 | TestEvaluate_exclude_no_match | ✅ |
| SVC-U-037 | TestEvaluate_include_matches | ✅ |
| SVC-U-038 | TestEvaluate_include_no_match | ✅ |
| SVC-U-039 | TestEvaluate_empty_text_with_include | ✅ |
| SVC-U-040 | TestEvaluate_submatch | ✅ |
| SVC-U-041 | TestEvaluate_submatch_no_match | ✅ |

---

### Service / Limiter (internal/service/limiter)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-042 | TestNew | ✅ |
| SVC-U-091 | TestWaitForForward_FirstCallNoWait | ✅ |
| SVC-U-092 | TestWaitForForward_SecondCallWaitsInterval | ✅ |
| SVC-U-093 | TestWaitForForward_ThirdCallWaitsAnotherInterval | ✅ |
| SVC-U-094 | TestWaitForForward_NoWaitAfterInterval | ✅ |
| SVC-U-095 | TestWaitForForward_WaitOnlyRemainingTime | ✅ |
| SVC-U-096 | TestWaitForForward_DifferentChatsIndependent | ✅ |
| SVC-U-097 | TestWaitForForward_ChatIDValues | ✅ |
| SVC-U-098 | TestWaitForForward_ContextCancelledReturnsEarly | ✅ |
| SVC-U-099 | TestWaitForForward_ContextAlreadyCancelledStillProceeds | ✅ |
| SVC-U-100 | TestWaitForForward_ContextCancelledDoesNotUpdateTimestamp | ✅ |
| SVC-U-101 | TestWaitForForward_ConcurrentSameChatCompletes | ✅ |
| SVC-U-102 | TestWaitForForward_SequentialSameChatSerialized | ✅ |
| SVC-U-103 | TestWaitForForward_ConcurrentDifferentChatsParallel | ✅ |

---

### Service / Message (internal/service/message)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-043 | TestGetFormattedText (5 subtests) | ✅ |
| SVC-U-044 | TestIsSystemMessage (3 subtests) | ✅ |
| SVC-U-045 | TestGetReplyMarkupData (3 subtests) | ✅ |
| SVC-U-046 | TestBuildInputContent_Photo | ✅ |
| SVC-U-047 | TestBuildInputContent_Text_InvertsLinkPreview | ✅ |
| SVC-U-048 | TestBuildInputContent_Document | ✅ |
| SVC-U-049 | TestBuildInputContent_VoiceNote | ✅ |
| SVC-U-050 | TestBuildInputContent_Video | ✅ |
| SVC-U-051 | TestBuildInputContent_Animation | ✅ |
| SVC-U-052 | TestBuildInputContent_Audio | ✅ |

---

### Service / Transform (internal/service/transform)

Файл: `service_test.go` ✅

| ID | Тест | Статус |
|----|------|--------|
| SVC-U-053 | TestTransform_NoTransformations | ✅ |
| SVC-U-054 | TestTransform_Translation | ✅ |
| SVC-U-055 | TestTransform_Translation_SkippedForOtherChat | ✅ |
| SVC-U-056 | TestTransform_ReplaceFragments | ✅ |
| SVC-U-057 | TestTransform_Sign | ✅ |
| SVC-U-058 | TestTransform_Link | ✅ |
| SVC-U-059 | TestTransform_PrevLink | ✅ |
| SVC-U-060 | TestAddNextLink | ✅ |
| SVC-U-061 | TestAddNextLink_NoNextConfig | ✅ |
| SVC-U-062 | TestAddNextLink_ChatNotInFor | ✅ |
| SVC-U-063 | TestEncodeDecodeUTF16 (4 subtests) | ✅ |
| SVC-U-064 | TestExtractSubstring | ✅ |
| SVC-U-065 | TestExtractSubstring_BeyondLength | ✅ |
| SVC-U-066 | TestReplaceFragment | ✅ |
| SVC-U-067 | TestReplaceFragment_NoMatch | ✅ |
| SVC-U-068 | TestReplaceFragment_NilText | ✅ |

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

## BDD-тесты (test/bdd/)

Требуют: TDLib (реальный клиент), авторизованная сессия, тестовые чаты из `cmd/stand`, LiveStack. Skip: `testing.Short()`.

Каждая feature содержит два типа сценариев:
- **Outline 01** — проверка через все типы исходных чатов (4 источника x все целевые)
- **Outline 02** — матрица конкретных пар источник-назначение (до 4x4=16 комбинаций)

Типы чатов: исходный публичный канал, исходный приватный канал, исходная публичная группа, исходная приватная группа, целевой публичный канал, целевой приватный канал, целевая публичная группа, целевая приватная группа.

### 01_delivery

Steps: `test/bdd/shared/steps_01_delivery.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-001 | 01_copy | 01. Копирование во все целевые чаты | ×4 |
| BDD-002 | 01_copy | 02. Копирование из конкретного источника в конкретный целевой чат | ×16 |
| BDD-003 | 02_forward | 01. Пересылка во все целевые чаты | ×4 |
| BDD-004 | 02_forward | 02. Пересылка из конкретного источника в конкретный целевой чат | ×16 |
| BDD-005 | 03_rate_limiting | 01. Не чаще раза в 3 секунды | ×1 |
| BDD-006 | 04_reply_chain | 01. Ответ сохраняет связь | ×1 |
| BDD-007 | 05_origin_unwrapping | 01. Разворачивание до оригинала | ×1 |
| BDD-008 | 06_statistics | 01. Счётчики просмотренных/пересланных | ×1 |
| BDD-009 | 07_system_messages | 01. Удаление при включённом флаге | ×1 |
| BDD-010 | 07_system_messages | 02. Игнорирование при выключенном | ×1 |

Итого 01_delivery: **46 scenario instances**

### 02_filters

Steps: `test/bdd/shared/steps_02_filters.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-011 | 01_exclude | 01. Без паттерна проходит (2 режима × 4 источника) | ×8 |
| BDD-012 | 01_exclude | 02. Фильтр исключения из конкретного источника в целевой чат (2 × 16) | ×32 |
| BDD-013 | 01_exclude | 03. С паттерном блокируется (2 × 4) | ×8 |
| BDD-014 | 01_exclude | 04. Блокировка из конкретного источника в целевой чат (2 × 16) | ×32 |
| BDD-015 | 02_include | 01. С разрешённым проходит (2 × 4) | ×8 |
| BDD-016 | 02_include | 02. Фильтр включения из конкретного источника в целевой чат (2 × 16) | ×32 |
| BDD-017 | 03_submatch | 01. Submatch-фильтр (2 × 4) | ×8 |
| BDD-018 | 03_submatch | 02. Submatch из конкретного источника в целевой чат (2 × 16) | ×32 |
| BDD-019 | 04_check_other_dedup | 01. В check-чат один раз | ×1 |

Итого 02_filters: **161 scenario instances**

### 03_transform

Steps: `test/bdd/shared/steps_03_transform.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-020 | 01_replace_own_links | 01. Замена ссылок (3 источника) | ×3 |
| BDD-021 | 01_replace_own_links | 02. Замена ссылок из источника в целевой чат (3 × 4) | ×12 |
| BDD-022 | 02_remove_external_links | 01. Удаление внешних (3 источника) | ×3 |
| BDD-023 | 02_remove_external_links | 02. Удаление внешних из источника в целевой чат (3 × 4) | ×12 |
| BDD-024 | 03_replace_fragments | 01. Замена фрагментов (4 источника) | ×4 |
| BDD-025 | 03_replace_fragments | 02. Замена фрагментов из источника в целевой чат (4 × 4) | ×16 |
| BDD-026 | 04_source_link | 01. Ссылка на оригинал (3 источника) | ×3 |
| BDD-027 | 04_source_link | 02. Ссылка на оригинал из источника в целевой чат (3 × 4) | ×12 |
| BDD-028 | 05_source_sign | 01. Подпись источника (4 источника) | ×4 |
| BDD-029 | 05_source_sign | 02. Подпись источника из источника в целевой чат (4 × 4) | ×16 |
| BDD-030 | 06_translate | 01. Перевод (4 источника) | ×4 |
| BDD-031 | 06_translate | 02. Перевод из источника в целевой чат (4 × 4) | ×16 |

Итого 03_transform: **105 scenario instances**

### 04_media

Steps: `test/bdd/shared/steps_04_media.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-032 | 01_album_copy | 01. Альбом копируется (4 источника) | ×4 |
| BDD-033 | 01_album_copy | 02. Копирование альбома из источника в целевой чат (4 × 4) | ×16 |
| BDD-034 | 02_album_forward | 01. Альбом пересылается (4 источника) | ×4 |
| BDD-035 | 02_album_forward | 02. Пересылка альбома из источника в целевой чат (4 × 4) | ×16 |

Итого 04_media: **40 scenario instances**

### 05_sync

Steps: `test/bdd/shared/steps_05_sync.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-036 | 01_versioning | 01. Новая версия со ссылками (3 источника) | ×3 |
| BDD-037 | 01_versioning | 02. Версионирование из источника в целевой чат (3 × 4) | ×12 |
| BDD-038 | 02_edit_update | 01. Обновление копии (4 источника) | ×4 |
| BDD-039 | 02_edit_update | 02. Обновление копии из источника в целевой чат (4 × 4) | ×16 |
| BDD-040 | 03_indelible | 01. Неудаляемые копии (4 источника) | ×4 |
| BDD-041 | 03_indelible | 02. Неудаляемые копии из источника в целевой чат (4 × 4) | ×16 |
| BDD-042 | 04_delete_sync | 01. Удаление копий (4 источника) | ×4 |
| BDD-043 | 04_delete_sync | 02. Синхронизация удаления из источника в целевой чат (4 × 4) | ×16 |
| BDD-044 | 05_retry | 01. Retry при отсутствии permanent ID | ×1 |

Итого 05_sync: **76 scenario instances**

### 06_auto

Steps: `test/bdd/shared/steps_06_auto.go`

| ID | Feature | Сценарий | Кол-во |
|----|---------|----------|--------|
| BDD-045 | 01_auto_answers | 01. Автоответ на callback (3 источника) | ×3 |
| BDD-046 | 01_auto_answers | 02. Автоответ из источника в целевой чат (3 × 4) | ×12 |

Итого 06_auto: **15 scenario instances**

---

## Смоук-тесты (test/smoke/)

Требуют: Docker, testcontainers-compose, Dockerfile. Build tag: `smoke`.

Файл: `smoke_test.go` ✅ (SmokeSuite через testify/suite)

| ID | Тест | Статус |
|----|------|--------|
| SMOKE-001 | TestHealthcheck | ✅ |
| SMOKE-002 | TestHealth | ✅ |
| SMOKE-003 | TestLive | ✅ |
| SMOKE-004 | TestReady | ✅ |

---

## Сводная таблица

| Пакет | Unit | BDD | Smoke |
|-------|------|-----|-------|
| internal/config | 4 | — | — |
| internal/controller | 5 | — | 4 |
| internal/domain | 7 | — | — |
| internal/handler | 20 | 443 | — |
| internal/repo/queue | 4 | — | — |
| internal/repo/ruleset | 11 | — | — |
| internal/repo/state | 11 | — | — |
| internal/repo/telegram | 23 | 443 | — |
| internal/repo/term | 6 | — | — |
| internal/service/album | 10 | 40 | — |
| internal/service/auth | 21 | — | — |
| internal/service/dedup | 10 | 1 | — |
| internal/service/facade | 14 | — | — |
| internal/service/filters | 8 | 161 | — |
| internal/service/limiter | 14 | 1 | — |
| internal/service/message | 10 | — | — |
| internal/service/transform | 16 | 105 | — |
| internal/transport/grpc | 18 | — | — |
| internal/transport/http | 12 | — | — |
| internal/transport/http/graph | 5 | — | — |
| internal/transport/term | 3 | — | — |
| **Итого** | **232** | **443 scenarios** | **4** |

---

## Покрытие кода

Снято командой `task cover`. Общее покрытие: **86.7%**.

| Пакет | Покрытие | Примечание |
|-------|----------|------------|
| internal/controller | 100.0% | health endpoints |
| internal/domain | 100.0% | типы, MaskPhoneNumber, DeepCopy |
| internal/handler | 100.0% | диспетчер обновлений |
| internal/repo/queue | 92.2% | in-memory queue |
| internal/repo/ruleset | 73.6% | YAML loader + валидация |
| internal/repo/state | 82.9% | BadgerDB CRUD + copies |
| internal/repo/telegram | 71.6% | TDLib clientAdapter + mapping; unit-тесты + BDD через LiveStack |
| internal/repo/term | 91.7% | readline adapter |
| internal/service/album | 100.0% | — |
| internal/service/auth | 100.0% | auth flow orchestration |
| internal/service/dedup | 100.0% | — |
| internal/service/facade | 96.3% | proxy + GetStatus |
| internal/service/filters | 100.0% | evaluate + submatch |
| internal/service/limiter | 100.0% | WaitForForward |
| internal/service/message | 100.0% | GetFormattedText, BuildInputContent |
| internal/service/transform | 99.4% | transform pipeline, UTF-16 |
| internal/transport/grpc | 74.1% | RPC-обёртки |
| internal/transport/http | 79.2% | REST auth |
| internal/transport/http/graph | — | gqlgen-generated код, исключён из отчёта |
| internal/transport/http/resolvers | 100.0% | GraphQL resolver |
| internal/transport/term | 51.0% | CLI auth + commands |
