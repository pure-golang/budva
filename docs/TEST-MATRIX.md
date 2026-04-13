# Test Matrix

## Сводка по слоям

| Слой | Пакетов | Тестов | Статус |
|---|---|---|---|
| unit | 5 | 18 | ✅ green |
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
| SVC-001 | internal/service/filters | TestEvaluate_* (8 тестов) | Все режимы фильтрации |
| SVC-002 | internal/service/dedup | TestTracker_* (3 теста) | Дедупликация доставки |
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

## Code Coverage

Пока не настроен — будет добавлен после заполнения реализаций стаб-сервисов.
