# Test Matrix

## Сводка по слоям

| Слой | Пакетов | Тестов | Статус |
|---|---|---|---|
| unit | 7 | 28 | ✅ green |
| integration | 0 | 0 | — |
| bdd | 1 | 56 | ✅ green (3 @pending) |
| e2e | 0 | 0 | — |
| smoke | 0 | 0 | — |

## Unit-тесты по пакетам

| ID | Пакет | Тест | Описание |
|---|---|---|---|
| CFG-001 | internal/config | TestTelegramConfig_RequiredFields | Обязательные поля Telegram |
| CFG-002 | internal/config | TestStorageConfig_Defaults | Дефолты хранилища |
| CFG-003 | internal/config | TestRulesetConfig_Defaults | Дефолт пути к ruleset |
| REPO-001 | internal/repo/queue | TestRepo_Add_and_Len | Добавление и подсчёт задач |
| REPO-002 | internal/repo/queue | TestRepo_ProcessQueue_executes_task | Выполнение задачи |
| REPO-003 | internal/repo/queue | TestRepo_ProcessQueue_recovers_from_panic | Recovery при панике |
| REPO-004 | internal/repo/queue | TestRepo_StartContext_processes_tasks | Обработка по таймеру |
| REPO-005 | internal/repo/state | TestRepo_SetGet | CRUD: запись + чтение |
| REPO-006 | internal/repo/state | TestRepo_Get_not_found | Отсутствующий ключ |
| REPO-007 | internal/repo/state | TestRepo_Delete | Удаление ключа |
| REPO-008 | internal/repo/state | TestRepo_GetSet_atomic | Атомарное чтение-изменение |
| REPO-009 | internal/repo/ruleset | TestRepo_Load | Загрузка YAML |
| REPO-010 | internal/repo/ruleset | TestRepo_Load_empty_config | Пустой конфиг |
| REPO-011 | internal/repo/ruleset | TestRepo_Load_file_not_found | Файл не найден |
| SVC-001 | internal/service/filters | TestEvaluate_no_filters | Без фильтров — OK |
| SVC-002 | internal/service/filters | TestEvaluate_exclude_matches | Exclude совпадает — Check |
| SVC-003 | internal/service/filters | TestEvaluate_exclude_no_match | Exclude не совпадает — OK |
| SVC-004 | internal/service/filters | TestEvaluate_include_matches | Include совпадает — OK |
| SVC-005 | internal/service/filters | TestEvaluate_include_no_match | Include не совпадает — Other |
| SVC-006 | internal/service/filters | TestEvaluate_empty_text_with_include | Пустой текст + Include — Other |
| SVC-007 | internal/service/filters | TestEvaluate_submatch | Submatch совпадает — OK |
| SVC-008 | internal/service/filters | TestEvaluate_submatch_no_match | Submatch не совпадает — Other |
| SVC-009 | internal/service/dedup | TestTracker_TryMark_first_time | Первая пометка |
| SVC-010 | internal/service/dedup | TestTracker_TryMark_duplicate | Дублирующая пометка |
| SVC-011 | internal/service/dedup | TestTracker_TryMark_unknown_chat | Неизвестный чат |
| CTRL-001 | internal/controller | TestLive_always_200 | /live всегда 200 |
| CTRL-002 | internal/controller | TestHealthcheck_all_healthy | Здоровый пинг |
| CTRL-003 | internal/controller | TestHealthcheck_unhealthy | Нездоровый пинг |

## BDD-сценарии

| Эпик | Файл | Сценариев | Статус |
|---|---|---|---|
| 01_delivery | 01_copy.feature | 4 | ✅ |
| 01_delivery | 02_forward.feature | 4 | ✅ |
| 02_filters | 01_exclude.feature | 16 | ✅ |
| 02_filters | 02_include.feature | 8 | ✅ |
| 02_filters | 03_submatch.feature | 8 | ✅ |
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
| 06_auto | 01_auto_answers.feature | 3 | ⏳ @pending |

## Пакеты без unit-тестов (tech debt)

| Пакет | Приоритет | Что покрывать |
|---|---|---|
| internal/handler | Высокий | OnNewMessage, OnEditedMessage, OnDeletedMessages — ядро бизнес-логики |
| internal/service/transform | Высокий | Transform pipeline, replaceMyselfLinks, UTF-16 |
| internal/service/message | Средний | GetFormattedText, BuildInputContent |
| internal/service/album | Средний | AddMessage, PopMessages, конкурентность |
| internal/service/auth | Средний | Subscribe, SetState, pub-sub |
| internal/service/limiter | Низкий | Wait — time-based логика |
| internal/repo/state/copies | Средний | SetCopiedMessageID, GetCopiedMessageIDs — string parsing |

## Code Coverage

Пока не настроен — будет добавлен после написания тестов для handler и transform.
