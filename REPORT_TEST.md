# Аудит тестового покрытия budva-claude

## Текущее состояние по слоям

| Слой | Расположение | Файлов | Функциональность |
|---|---|---|---|
| Unit | `*_test.go` рядом с кодом | 15 | Работают, покрывают компоненты изолированно |
| BDD | `test/bdd/` | 18 features + 6 step-файлов | **Заглушки** — steps не вызывают реальный код |
| Integration | `test/integration/` | 0 | Пусто |
| E2E | `test/e2e/` | — | Директория не существует |
| Smoke | `test/smoke/` | 0 | Пусто |

## Unit-тесты — что покрыто

| Пакет | Тестов | Что проверяется |
|---|---|---|
| handler | 12 | OnNewMessage (7 сценариев), OnEditedMessage, OnDeletedMessages (retry, indelible), OnMessageSendSucceeded, SetRuleSet |
| transform | 16 | Transform pipeline (7 шагов), AddNextLink, UTF-16, replaceFragment |
| message | 16 | GetFormattedText, IsSystemMessage, GetReplyMarkupData, BuildInputContent (4 типа) |
| auth | 8 | Subscribe, SetState, Extra, InputChan, ReadInput, concurrency |
| album | 10 | AddMessage, PopMessages, LastReceivedAge, MakeKey |
| filters | 8 | Evaluate: exclude, include, submatch, empty text |
| dedup | 3 | TryMark, dedup per destination |
| config | 3 | envconfig loading |
| controller | 3 | healthcheck, live, unhealthy |
| repo/state | 4 | BadgerDB get/set/increment |
| repo/ruleset | 3 | YAML load, validate, transform/enrich |
| repo/queue | 4 | Add, processQueue, panic recovery, StartContext |
| transport/http | 12 | REST auth endpoints (state, phone, code, password, hint) |
| transport/grpc | 19 | FacadeGRPC все 10 RPC + helpers |
| transport/http/graph | 5 | GraphQL handler, playground |

**Итого: ~126 unit-тестов, 15 пакетов**

## Unit-тесты — что НЕ покрыто

| Пакет | Что не тестируется |
|---|---|
| handler | editMessages (полный путь с transform), forwardMessage (SendCopy путь с origin), resolveReplyTo, getOriginMessage, processMediaAlbum, statistics, runNextLinkWorkflow |
| transform | replaceMyselfLinks (complex link rewriting), addAutoAnswer |
| limiter | WaitForForward (timing-based) |
| repo/state | copies.go (SetCopiedMessageID update-in-place logic) |
| facade | Все методы (0 тестов) |

## BDD — состояние сценариев

### Покрытые бизнес-функции (18 scenarios)

| Эпик | Feature | Сценариев | Step status |
|---|---|---|---|
| 01_DELIVERY | copy, forward | 2 | Заглушка |
| 02_FILTERS | exclude, include, submatch | 4 | Заглушка |
| 03_TRANSFORM | replace links, remove external, fragments, source link, sign, translate | 6 | Заглушка |
| 04_MEDIA | album copy, album forward | 2 | Заглушка |
| 05_SYNC | versioning, edit update, indelible, delete sync | 4 | Заглушка |
| 06_AUTO | auto answers | 1 | @pending |

### НЕ покрытые бизнес-функции

| Функция | Реализована в коде | BDD сценарий |
|---|---|---|
| Rate limiting (3s per chat) | ✓ handler + limiter | ✗ Нет |
| Statistics (viewed/forwarded) | ✓ handler | ✗ Нет |
| Reply chain preservation | ✓ handler.resolveReplyTo | ✗ Нет |
| Origin message unwrapping | ✓ handler.getOriginMessage | ✗ Нет |
| Check/Other dedup | ✓ handler via DedupTracker | ✗ Нет |
| Retry 3x on edit/delete | ✓ handler.editMessagesWithRetry | ✗ Нет |
| Password hint in HTTP | ✓ transport/http | ✗ Нет |
| Fragment UTF-16 validation | ✓ ruleset validate | ✗ Нет |

### Проблема: BDD steps не выполняют реальный код

Все step definitions — in-memory state tracking:

```go
// Так выглядит "тест" сейчас:
func (s *scenarioCtx) messageIsDeliveredToAllTargets() error {
    s.delivered = true  // ← просто ставит флаг
    return nil
}
```

Ни один step не создаёт `Handler`, `RuleSet`, `Service` и не вызывает бизнес-логику. Тесты проходят вне зависимости от корректности кода.

## Integration тесты — пусто

Директория `test/integration/` существует, но пуста. Нужны тесты для:

| Компонент | Что тестировать |
|---|---|
| repo/state (BadgerDB) | SetCopiedMessageID update-in-place, Increment atomicity, GetSet transaction |
| repo/ruleset | Load → validate → transform → enrich → check полный цикл с реальным YAML |
| handler + services | Полный forwarding pipeline с реальными service implementations |

## E2E тесты — не существует

Директория `test/e2e/` отсутствует. При наличии TDLib stubs можно тестировать:

| Сценарий | Описание |
|---|---|
| cmd/engine startup | Запуск → загрузка конфига → подписка на updates → shutdown |
| cmd/facade startup | Запуск → HTTP endpoints доступны → gRPC reflection → shutdown |
| Forwarding pipeline | Update → handler → transform → send → storage mapping |

## Smoke тесты — пусто

Директория `test/smoke/` существует, но пуста. Нужны:

| Проверка | Описание |
|---|---|
| Binary builds | `go build ./cmd/engine` и `./cmd/facade` |
| Config loads | Минимальный `.env` → `InitConfig` без ошибок |
| Health endpoint | HTTP GET /live → 200 |

## План закрытия gaps

### Приоритет 1: Сделать BDD functional (red→green)

Подключить реальный `Handler` + services в BDD step definitions. Для этого нужен `test/support/` с:
- `TestHandler` — собранный handler с mockery-моками для telegram
- `TestRuleSet` — fixture-набор правил
- `TestContext` — shared state между steps

**Результат:** 18 существующих сценариев будут проверять реальную бизнес-логику.

### Приоритет 2: Добавить BDD для новых функций

7 новых .feature файлов:
- `05_SYNC/05_retry_eventual_consistency.feature`
- `01_DELIVERY/03_rate_limiting.feature`
- `01_DELIVERY/04_reply_chain.feature`
- `01_DELIVERY/05_origin_unwrapping.feature`
- `01_DELIVERY/06_statistics.feature`
- `02_FILTERS/04_check_other_dedup.feature`
- `06_AUTO/02_auto_answers_functional.feature` (замена @pending)

### Приоритет 3: Integration тесты

- `test/integration/state_copies_test.go` — BadgerDB update-in-place
- `test/integration/ruleset_load_test.go` — полный YAML pipeline
- `test/integration/handler_pipeline_test.go` — handler + real services

### Приоритет 4: E2E с TDLib stubs

- `test/e2e/engine_lifecycle_test.go` — startup → process updates → shutdown
- `test/e2e/facade_endpoints_test.go` — HTTP + gRPC endpoints functional

### Приоритет 5: Smoke тесты

- `test/smoke/build_test.go` — бинарники собираются
- `test/smoke/config_test.go` — конфиг загружается
- `test/smoke/health_test.go` — health endpoint отвечает
