# Отчёт о миграции budva43 → budva-claude

## Текущее состояние

| Метрика | Значение |
|---|---|
| Go-файлов | 63 (без тестов, моков, protobuf) + 22 тестовых |
| Unit-тестов | ~120 (15 пакетов) |
| BDD-сценариев | 19 (18 active + 1 @pending) |
| `go build ./...` | OK |
| `go test ./...` | OK (все 15 пакетов) |

## Что перенесено

### Полностью реализовано

| Область | Legacy-пакет | Новый пакет | Статус |
|---|---|---|---|
| Domain-модель | `app/domain/` | `internal/domain/` | Расширена: Message, FormattedText, TextEntity, Auth, Update |
| Config | `app/config/` | `internal/config/` | Viper → envconfig, 3 теста |
| In-process очередь | `repo/queue/` | `internal/repo/queue/` | Полная реализация, 4 теста |
| BadgerDB хранилище | `repo/storage/` + `service/storage/` | `internal/repo/state/` | Объединены в один пакет, 4 теста + 14 domain-level методов |
| YAML-загрузчик правил | `app/engine_config/` | `internal/repo/ruleset/` | Viper → yaml.v3, fsnotify watcher, validate/transform/enrich, 3 теста |
| Фильтрация сообщений | `service/filters_mode/` | `internal/service/filters/` | Полная логика: exclude/include/submatch, 8 тестов |
| Дедупликация | `service/forwarded_to/` | `internal/service/dedup/` | Tracker + DedupFactory, 3 теста |
| Rate limiter | `service/rate_limiter/` | `internal/service/limiter/` | Полная реализация |
| Медиа-альбомы | `service/media_album/` | `internal/service/album/` | Полная реализация (domain-типы вместо go-tdlib) |
| Auth state machine | `service/auth/` | `internal/service/auth/` | Pub-sub, InputChan |
| Извлечение контента | `service/message/` | `internal/service/message/` | GetFormattedText, IsSystemMessage, BuildInputContent |
| Трансформации | `service/transform/` | `internal/service/transform/` | Translate, replaceMyselfLinks, replaceFragments, sign, link, prev/next, UTF-16 |
| Обработчики обновлений | `handler/update_*` (4 пакета) | `internal/handler/` | Объединены в один пакет с consumer-side интерфейсами |
| Health controller | — | `internal/controller/` | По эталону geo, 3 теста |
| BDD-сценарии | `test/e2e/feature/` (21 файл) | `test/bdd/features/` (18 файлов в 6 эпиках) | Harvested и переписаны на бизнес-язык |

### Стабы (интерфейс определён, реализация — заглушка)

| Область | Legacy-пакет | Новый пакет | Что сделано | Что осталось |
|---|---|---|---|---|
| Telegram API | `repo/telegram/` | `internal/repo/telegram/` | 20+ методов-стабов, полный интерфейс | Реальная интеграция с go-tdlib |
| Terminal transport | `transport/term/` | `internal/transport/term/` | Структура + Run(ctx) | Auth flow, CLI команды |
| HTTP transport | `transport/web/` | `internal/transport/http/` | REST auth endpoints (4 маршрута), 11 тестов | GraphQL, playground |
| gRPC transport | `transport/grpc/` | `internal/transport/grpc/` | Proto, FacadeGRPC (10 RPC), 18 тестов | GetChatHistory (unimplemented) |
| Терминальный I/O | `repo/term/` | `internal/repo/term/` | ReadLine, ReadPassword, Print | — |

### Заглушки-сервисы (конструктор + logger, без бизнес-логики)

| Пакет | Причина |
|---|---|
| `internal/service/engine/` | Диспетчеризация вынесена в cmd/engine |
| `internal/service/forwarder/` | Логика пересылки вынесена в handler |
| `internal/service/loader/` | Загрузка ruleset вынесена в cmd/engine |

## Открытые findings от ревьюверов (раунд 4)

### DoD-reviewer

| # | Серьёзность | Описание |
|---|---|---|
| 1 | Средняя | Ключ ошибки `"error"` вместо `"err"` — 33 места. Проверить, что skill x-log действительно требует `"err"`, и унифицировать |
| 2 | Низкая | 2 места с нетипизированными slog-атрибутами |
| 3 | Низкая | `slog.Default()` в defer closeMonitoring вместо `logger` |
| 4 | Info | 5 неиспользуемых tracer (подготовка к инструментации) |
| 5 | Низкая | .env.example не содержит переменных мониторинга |

### BDD-reviewer

| # | Серьёзность | Описание |
|---|---|---|
| 1 | Низкая | Step-файлы без числового префикса (конфликт x-bdd-godog vs revive linter) |
| 2 | Средняя | @pending в auto_steps — by design, ждёт TDLib |

### Test-reviewer

Нарушений нет.

## Дальнейший план работ

### ~~Приоритет 1: Unit-тесты для ядра бизнес-логики~~ ✓

Реализовано: mockery v2 конфиг, моки для handler/transform/controller, unit-тесты для handler (16), transform (16), message (13), auth (8), album (10).

### ~~Приоритет 2: Transport-слой~~ ✓

Реализовано:
- `internal/transport/http/` — REST auth endpoints (4 маршрута, 11 тестов)
- `internal/transport/grpc/` — Proto + FacadeGRPC (10 RPC, 18 тестов)
- `internal/service/facade/` — расширен из заглушки до полной реализации с telegramGateway

Остаётся: GraphQL handler + playground (HTTP), GetChatHistory (gRPC).

### Приоритет 3: Observability

| Задача | Описание |
|---|---|
| Span-инструментация | Подключить tracer.Start/span.End в repo/state, repo/ruleset, service/transform, service/filters |
| Logging audit | Унифицировать ключ ошибки (`"error"` vs `"err"` по x-log) |
| Monitoring env | Документировать переменные monitoring.Config в doc.go и .env.example |

### Приоритет 4: Интеграция с TDLib (фаза 11)

| Задача | Описание |
|---|---|
| go-tdlib dependency | CGO + TDLib C-библиотека |
| `repo/telegram/` | Заменить стабы реальными вызовами TDLib |
| Auth flow | Реальная авторизация через Test DC |
| Update loop | Подключить TDLib Listener к update dispatcher |
| E2E тесты | Против Telegram Test DC |

### Приоритет 5: Конфликт x-bdd-godog vs linter

Решить: либо обновить x-bdd-godog skill (убрать числовые префиксы из step-файлов), либо настроить исключение в golangci-lint для `test/bdd/steps/`.
