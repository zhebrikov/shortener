# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Сборка с метаданными версии

При запуске `cmd/shortener` в stdout выводятся версия, дата и коммит сборки. По умолчанию для всех трёх полей используется `N/A`; при сборке их можно задать через `-ldflags` и флаг `-X` компилятора Go (значения переменных уровня пакета в `main` перезаписываются на этапе линковки).

Сборка с подстановкой метаданных:

```bash
go build -ldflags "\
  -X main.buildVersion=1.2.3 \
  -X main.buildDate=2024-01-15T12:00:00Z \
  -X 'main.buildCommit=abc123def'" \
  -o shortener ./cmd/shortener
```

Дату сборки удобно подставлять из shell:

```bash
go build -ldflags "\
  -X main.buildVersion=$(git describe --tags --always) \
  -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -X main.buildCommit=$(git rev-parse --short HEAD)" \
  -o shortener ./cmd/shortener
```

Те же флаги работают с `go run`:

```bash
go run -ldflags "-X main.buildVersion=dev -X main.buildDate=local -X main.buildCommit=none" ./cmd/shortener
```

Имена переменных (`main.buildVersion`, `main.buildDate`, `main.buildCommit`) должны совпадать с объявлением в `cmd/shortener/main.go`. Значения с пробелами заключайте в кавычки, как в примере с `buildCommit`.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Бенчмарки

Бенчмарки для ключевых компонентов (сервис сокращения, in-memory хранилище, HTTP-обработчики):

```bash
go test -bench=. -benchmem ./internal/service/... ./internal/storage/... ./internal/handler/...
```

Пример результата (darwin/arm64):

| Бенчмарк | ns/op | B/op | allocs/op |
|----------|------:|-----:|----------:|
| `BenchmarkShortener_CreateLink` | 650162 | 319737 | 14908 |
| `BenchmarkShortener_GetLink` | 22.75 | 0 | 0 |
| `BenchmarkMemoryStorage_ReadStorage` | 68705 | 327680 | 1 |
| `BenchmarkMemoryStorage_GetLinkByShortCode` | 18.00 | 0 | 0 |
| `BenchmarkMemoryStorage_WriteStorage` | 124200 | 455 | 4 |
| `BenchmarkCreateLinkJSON` | 785989 | 567887 | 19940 |
| `BenchmarkGetLink` | 1744 | 6060 | 17 |
| `BenchmarkCreateLink_plain` | 737447 | 567260 | 19934 |

| Бенчмарк | Назначение |
|----------|------------|
| `BenchmarkShortener_CreateLink` / `GetLink` | генерация short code и редирект |
| `BenchmarkMemoryStorage_ReadStorage` / `GetLinkByShortCode` / `WriteStorage` | чтение и поиск в памяти |
| `BenchmarkCreateLinkJSON` / `GetLink` / `CreateLink_plain` | HTTP-слой |

## Профилирование памяти (pprof)

### Снятие профилей

Нагрузка и запись heap-профиля (`cmd/memprofile` прогоняет создание и чтение ссылок, затем `runtime.GC()` и `pprof.WriteHeapProfile`):

```bash
./scripts/capture-profiles.sh
```

Скрипт временно подставляет baseline-код из `tools/profilebaseline/`, пишет `profiles/base.pprof`, восстанавливает оптимизированный код и пишет `profiles/result.pprof`.

Вручную:

```bash
go run ./cmd/memprofile -output profiles/base.pprof
go run ./cmd/memprofile -output profiles/result.pprof
```

### Анализ baseline (`profiles/base.pprof`)

Для поиска «тяжёлых» функций по суммарным аллокациям за прогон используйте `-alloc_space` (после `runtime.GC()` в снимке `inuse_space` почти не видно кода приложения):

```bash
go tool pprof -top -alloc_space profiles/base.pprof
go tool pprof -list=ReadStorage -alloc_space profiles/base.pprof
go tool pprof -peek=GetLink -alloc_space profiles/base.pprof
go tool pprof -web -alloc_space profiles/base.pprof
```

Фрагмент `top` (baseline):

```
      flat  flat%   sum%        cum   cum%
 4096.31MB 52.22% 52.22%  4096.31MB 52.22%  ...MemoryStorage).ReadStorage
 2306.61MB 29.40% 81.62%  2306.61MB 29.40%  encoding/hex.EncodeToString
  761.02MB  9.70% 91.33%   764.53MB  9.75%  fmt.Sprintf
  559.05MB  7.13% 98.45%  3635.18MB 46.34%  ...Shortener).CreateLink
```

`peek GetLink` показывает, что ~1.9 GB аллокаций уходит в `ReadStorage` при каждом редиректе.

### Сравнение профилей

Команда из задания (удерживаемая память в момент снимка, `inuse_space`):

```bash
go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
```

Суммарные аллокации за прогон (основной показатель оптимизации):

```bash
go tool pprof -top -alloc_space -diff_base=profiles/base.pprof profiles/result.pprof
```

Вывод (отрицательные значения — меньше аллокаций в оптимизированной версии):

```
File: memprofile
Type: alloc_space
Time: 2026-05-19 08:08:21 MSK
Showing nodes accounting for -6151.41MB, 78.42% of 7844.38MB total
Dropped 65 nodes (cum <= 39.22MB)
      flat  flat%   sum%        cum   cum%
-4096.31MB 52.22% 52.22% -4096.31MB 52.22%  github.com/zhebrikov/shortener/internal/storage.(*MemoryStorage).ReadStorage
-2306.61MB 29.40% 81.62% -2306.61MB 29.40%  encoding/hex.EncodeToString (inline)
 1174.55MB 14.97% 66.65%  1174.55MB 14.97%  internal/bytealg.MakeNoZero
 -761.02MB  9.70% 76.35%  -764.53MB  9.75%  fmt.Sprintf
 -558.53MB  7.12% 83.47% -2057.60MB 26.23%  github.com/zhebrikov/shortener/internal/service.(*Shortener).CreateLink
  203.50MB  2.59% 80.88%   203.50MB  2.59%  internal/strconv.FormatInt
     196MB  2.50% 78.38%      196MB  2.50%  github.com/zhebrikov/shortener/internal/service.shortCodeFromHash (inline)
      -2MB 0.025% 78.41% -6174.50MB 78.71%  main.main
   -0.50MB 0.0064% 78.41% -2020.49MB 25.76%  github.com/zhebrikov/shortener/internal/handler.CreateLink
   -0.50MB 0.0064% 78.42% -1894.02MB 24.14%  github.com/zhebrikov/shortener/internal/service.(*Shortener).GetLink
         0     0% 78.42% -1950.43MB 24.86%  github.com/zhebrikov/shortener/internal/handler.CreateLinkJSON
         0     0% 78.42%  -949.26MB 12.10%  github.com/zhebrikov/shortener/internal/handler.GetLink
         0     0% 78.42%  -289.48MB  3.69%  github.com/zhebrikov/shortener/internal/storage.(*MemoryStorage).NextLinkUUID
```

Итого за прогон: **−4096 MB** `ReadStorage`, **−2307 MB** `hex.EncodeToString`, **−761 MB** `fmt.Sprintf`, **−559 MB** в `CreateLink`.

### Что оптимизировано

1. **Поиск ссылки** — `GetLinkByShortCode` с индексом `byCode` в `MemoryStorage` вместо `ReadStorage` + линейный обход; `GetLink` в сервисе делегирует в хранилище.
2. **UUID при создании** — `NextLinkUUID()` читает только последний UUID под `RLock`, без копии всего слайса.
3. **Short code** — `shortCodeFromHash` (буфер `[8]byte` + `hex.Encode`) вместо `hex.EncodeToString`; `strings.Builder` вместо `fmt.Sprintf` при коллизиях.
