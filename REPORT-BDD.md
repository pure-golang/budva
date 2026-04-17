# BDD Performance: анализ и план ускорения

## Текущие метрики

| Метрика | Значение |
|---|---|
| Общее время | **~20 мин** (1211s) |
| Компиляция (`-race` + TDLib CGO) | ~6 мин (360s) |
| Выполнение тестов | ~14 мин (850s) |
| Сценариев | 443 |
| Среднее на сценарий | ~1.9s |
| Результат | 441 passed, 2 failed |

## Распределение сценариев

| Tier | Описание | Сценариев | % от total | Время (оценка) |
|---|---|---|---|---|
| Simple | Единичные scenarios (rate limit, reply, dedup) | 7 | 2% | ~13s |
| 01 outlines | 4 типа источника (pub chl, prv chl, pub grp, prv grp) | ~72 | 16% | ~140s |
| 02 matrix | 16 пар source×target | ~288 | 65% | ~550s |
| Filters extra | exclude/include: 2 режима (copy/forward) × 4/16 | ~76 | 17% | ~145s |

**65% времени** уходит на матрицу source×target (02 outlines). Каждая из 18 features имеет outline с 16 примерами. Все 16 пар тестируют одну и ту же логику handler-а — различается только тип чата.

## Где теряется время

### 1. Компиляция: 6 мин

`-race` с TDLib CGO линковкой — основная причина. Race detector инструментирует весь код включая CGO мост.

### 2. Telegram API latency: ~1-3s на сценарий

Каждый сценарий:
- `PutMessage` → `SendMessageAndWait` → ~1-2s (отправка + ожидание permanent ID)
- `CheckLastMessage` → поллинг GetChatHistory каждые 500ms → ~0.5-2s
- `CheckNoMessage` → фиксированный sleep 3s
- Edit step → `EditMessageText` + 2s sleep + DrainQueue

### 3. Redundant matrix: 288 сценариев для 18 features

Матрица source×target (4×4 = 16 пар) верифицирует что handler корректно работает с разными типами чатов. Но:
- Handler не различает типы чатов — он вызывает одинаковый `SendMessage`/`ForwardMessages` для всех
- Различия только на уровне TDLib: message links не работают для basic groups, edit permissions разные для каналов vs групп
- Эти TDLib-ограничения можно проверить ОДНИМ сценарием для каждого типа, а не 16 раз для каждой feature

## Стратегии ускорения

### S1. Убрать `-race` из локального `task bdd` (~6 мин → 0)

**Экономия: ~6 мин (30% от total)**

Race detector нужен в CI, не при каждом локальном запуске. Создать два task:

```yaml
bdd:
  desc: "Run BDD scenarios (fast, no race)"
  cmd: go test -count=1 -p 8 -timeout 30m ./test/bdd/steps/...

bdd-race:
  desc: "Run BDD scenarios with race detector"
  cmd: go test -race -count=1 -p 8 -timeout 30m ./test/bdd/steps/...
```

### S2. godog tags: `@smoke` vs `@matrix` (~14 мин → 3 мин)

**Экономия: ~11 мин (55% от total)**

Разделить сценарии на два уровня:
- `@smoke` — 01 outlines (4 примера) + simple scenarios = ~79 сценариев
- `@matrix` — 02 outlines (16 пар) = ~288 сценариев

```gherkin
@smoke
Scenario Outline: 01. Сообщение копируется во все целевые чаты
  ...

@matrix
Scenario Outline: 02. Копирование из конкретного источника в конкретный целевой чат
  ...
```

Taskfile:
```yaml
bdd:
  cmd: go test -count=1 -timeout 10m ./test/bdd/steps/... -godog.tags=@smoke

bdd-full:
  cmd: go test -count=1 -timeout 30m ./test/bdd/steps/...
```

Локально — `@smoke` (79 сценариев, ~3 мин). CI — full (443 сценария, ~20 мин).

### S3. Уменьшить polling timeouts

**Экономия: ~1-2 мин**

| Параметр | Сейчас | Предложение | Обоснование |
|---|---|---|---|
| `CheckLastMessage` deadline | 10s | 5s | Сообщения обычно появляются за 1-2s |
| `CheckLastMessage` poll interval | 500ms | 200ms | Быстрее обнаруживаем сообщение |
| `CheckNoMessage` sleep | 3s | 2s | 2s достаточно для проверки отсутствия |
| Edit step sleep | 2s | 1s | UpdateMessageEdited приходит за <500ms |
| `PutAlbum` deadline | 15s | 8s | Альбомы обычно появляются за 2-3s |

### S4. Сократить матрицу фильтров

**Экономия: ~2 мин**

`02_filters/01_exclude` имеет 4 outline (01, 02, 03, 04) вместо 2. Сценарии 03 и 04 — это "блокировка" с паттерном, которые можно объединить с 01/02 или протестировать с меньшим количеством примеров.

Вместо 80 сценариев фильтров → ~40 (убрать 04 matrix, оставить 03 simple).

### S5. Pre-warm TDLib кеш

**Экономия: ~30s**

В `LiveStack.Start()` после авторизации — вызвать `LoadChats` + `WarmUpChat` для всех test чатов. Это загрузит историю в TDLib кеш и ускорит первые `GetChatHistory` вызовы.

### S6. Параллельные сценарии (godog concurrency)

**Потенциал: 2-4× ускорение, но сложно**

godog поддерживает `Concurrency: N`. Но:
- Все сценарии используют один TDLib клиент → одни чаты
- `prefix` уникален для каждого сценария, но сообщения в чатах перемешиваются
- `ResetState` между сценариями конфликтует с параллельным выполнением
- Нужна изоляция: каждый параллельный worker использует свой набор чатов

**Если делать**: создать 2-4 набора тестовых чатов через `cmd/stand`, запускать godog с `Concurrency: 4`, маршрутизировать сценарии по наборам чатов.

**Рекомендация**: не делать сейчас — слишком инвазивно. S1+S2 дают достаточный результат.

## Комбинированный результат

| Стратегия | Экономия | Сложность |
|---|---|---|
| S1. Убрать `-race` локально | 6 мин | Trivial |
| S2. `@smoke`/`@matrix` tags | 11 мин | Low |
| S3. Уменьшить timeouts | 1-2 мин | Low |
| S4. Сократить фильтры | 2 мин | Low |
| S5. Pre-warm cache | 30s | Low |
| **Итого S1-S5** | **~20 мин → ~3 мин** | |

### Целевые метрики

| Режим | Сценариев | Время |
|---|---|---|
| `task bdd` (smoke, no race) | ~79 | **~2 мин** |
| `task bdd-full` (all, no race) | ~443 | **~14 мин** |
| `task bdd-race` (all, race) — CI | ~443 | **~20 мин** |

## Рекомендуемый порядок реализации

1. **S1** — два task в Taskfile (5 мин работы, 6 мин экономии)
2. **S2** — godog tags + godog.tags filter (30 мин работы, 11 мин экономии)
3. **S3** — уменьшить timeouts (15 мин работы)
4. **S5** — pre-warm в Start() (10 мин работы)
5. **S4** — если нужно ещё быстрее
