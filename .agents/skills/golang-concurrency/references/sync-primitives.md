# Примитивы синхронизации

## sync.Mutex

Защищает общий инвариант исключительным доступом. Держи критическую секцию
достаточно короткой; не выполняй медленный I/O под блокировкой без причины.
Механический перенос I/O наружу также может нарушить нужную сериализацию.

```go
type SafeCache struct {
    mu    sync.Mutex
    items map[string]string
}

func (c *SafeCache) Get(key string) (string, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    v, ok := c.items[key]
    return v, ok
}

func (c *SafeCache) Set(key, value string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[key] = value
}
```

### Размещение mutex

Именованное неэкспортируемое поле не открывает `Lock` и `Unlock` как часть API.
Держи его рядом с защищаемыми полями.

```go
type Registry struct {
    mu      sync.Mutex // protects entries
    entries map[string]Entry
}
```

## sync.RWMutex

Позволяет одновременное чтение под `RLock`, но запись требует исключительного
`Lock`. Имеет смысл при подходящем профиле нагрузки; не гарантирует ускорения.

```go
type Config struct {
    mu     sync.RWMutex
    values map[string]string
}

func (c *Config) Get(key string) string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.values[key]
}

func (c *Config) Set(key, value string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.values[key] = value
}
```

Нельзя повышать `RLock` до `Lock` без освобождения. Между освобождением и новой
блокировкой состояние может измениться: при необходимости проверь условие заново.

## sync/atomic

Для независимых счётчиков и флагов atomic может быть проще mutex.
Он не обеспечивает инвариант сразу между несколькими значениями.

```go
// ✓ Good — atomic for a simple counter
var requestCount atomic.Int64

func handleRequest() {
    requestCount.Add(1)
}

func getCount() int64 {
    return requestCount.Load()
}
```

Для атомарной публикации указателя:

```go
// ✓ Good — atomic.Bool for a shutdown flag
var shuttingDown atomic.Bool

func shutdown() {
    shuttingDown.Store(true)
}

func isRunning() bool {
    return !shuttingDown.Load()
}
```

В Go 1.19+ предпочитай типизированные `atomic.Int64`, `atomic.Bool`, `atomic.Pointer[T]`.

## sync.Map

Подходит для записи один раз с многократным чтением или работы с непересекающимися
наборами ключей. В обычном случае начни с map и согласованной блокировки.

```go
var cache sync.Map

func Get(key string) (any, bool) {
    return cache.Load(key)
}

func Set(key string, value any) {
    cache.Store(key, value)
}

func GetOrSet(key string, compute func() any) any {
    if v, ok := cache.Load(key); ok {
        return v
    }
    v, _ := cache.LoadOrStore(key, compute())
    return v
}
```

`Range` не является согласованным снимком; составные операции и связанные ключи
требуют собственного протокола. Выбор ради скорости подтверждай измерением.

## sync.Pool

Переиспользует временные объекты для снижения числа выделений памяти.
Объект может быть удалён из пула сборщиком мусора: не храни там обязательное
состояние или ресурс, который требует гарантированного закрытия.

```go
var bufPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

func process(data []byte) string {
    buf := bufPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufPool.Put(buf)
    }()

    buf.Write(data)
    // ... transform ...
    return buf.String()
}
```

Очищай состояние перед повторным использованием; `Get` не гарантирует нулевое
значение. `New` вызывается не при каждом получении. Указатели на локальные
значения при сохранении могут уйти в heap через escape analysis; запрета языка
на такие указатели нет. Важны владение и отсутствие доступа после `Put`.

## sync.Once

Выполняет инициализацию один раз независимо от числа конкурентных вызовов.
Выбирай его только при таком контракте: повторить неудачную инициализацию нельзя.

```go
type DBClient struct {
    initOnce  sync.Once
    closeOnce sync.Once
    conn      *sql.DB
}

func (c *DBClient) getConn() *sql.DB {
    c.initOnce.Do(func() {
        var err error
        c.conn, err = sql.Open("postgres", dsn)
        if err != nil {
            panic(fmt.Sprintf("db init: %v", err))
        }
    })
    return c.conn
}

func (c *DBClient) Close() error {
    var err error
    c.closeOnce.Do(func() {
        err = c.conn.Close()
    })
    return err
}
```

Go 1.21+ предоставляет `OnceFunc`, `OnceValue` и `OnceValues`:

```go
var loadConfig = sync.OnceValue(func() *Config {
    cfg, err := parseConfig("config.yaml")
    if err != nil {
        panic(err)
    }
    return cfg
})

// Usage: cfg := loadConfig()
```

## sync.WaitGroup

Ожидает конечную группу работ, но не переносит ошибки и не отменяет соседние задачи.

### WaitGroup.Go: Go 1.25+

Запускает функцию в горутине и сам ведёт учёт её завершения.

```go
func processAll(items []Item) {
    var wg sync.WaitGroup

    for _, item := range items {
        // Go 1.22+ loop variables are per-iteration when the module has `go 1.22+`.
        // Do not add `item := item` solely for closure capture in modern modules.
        wg.Go(func() {
            process(item)
        })
    }

    wg.Wait()
}
```

Переданная функция не должна вызывать panic. Для первой ошибки, общей отмены
или ограничения конкурентности рассмотри `errgroup`. `wg.Go` устраняет ручные
`Add`/`Done` и связанные ошибки порядка, но не заменяет жизненный цикл задачи.

### Для более ранних версий Go

```go
func processAll(ctx context.Context, items []Item) {
    var wg sync.WaitGroup
    for _, item := range items {
        wg.Add(1) // Add BEFORE go
        go func(item Item) {
            defer wg.Done()
            process(ctx, item)
        }(item)
    }
    wg.Wait() // blocks until all goroutines finish
}
```

`Add` должен происходить до запуска горутины, а не внутри неё:

```go
// ✗ Bad — Add inside the goroutine (race: Wait may return before Add runs)
go func() {
    wg.Add(1)
    defer wg.Done()
    process(item)
}()
```

## singleflight

Объединяет одновременные вызовы с одним ключом: один выполняется, остальные
ждут и разделяют результат. Полезен против одновременного заполнения кэша,
дорогих повторных запросов к БД или ограниченному внешнему API.

```go
var group singleflight.Group

func GetUser(ctx context.Context, id string) (*User, error) {
    v, err, _ := group.Do(id, func() (any, error) {
        // Only one goroutine executes this for a given id
        return db.QueryUser(ctx, id)
    })
    if err != nil {
        return nil, err
    }
    return v.(*User), nil
}
```

Он не заменяет кэш и сам по себе не определяет срок хранения результата.

## errgroup

Возвращает первую ошибку. Вариант с `WithContext` отменяет дочерний контекст
при первой ошибке; сами функции должны учитывать отмену.

```go
func fetchAll(ctx context.Context, urls []string) ([]Response, error) {
    g, ctx := errgroup.WithContext(ctx) // cancel siblings on first error
    results := make([]Response, len(urls))

    for i, url := range urls {
        g.Go(func() error {
            resp, err := fetch(ctx, url)
            if err != nil {
                return fmt.Errorf("fetching %s: %w", url, err)
            }
            results[i] = resp // safe: each goroutine writes to its own index
            return nil
        })
    }

    if err := g.Wait(); err != nil {
        return nil, err
    }
    return results, nil
}
```

### SetLimit

Ограничивает число активных функций группы.

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(10) // at most 10 goroutines run concurrently

for _, task := range tasks {
    g.Go(func() error {
        return process(ctx, task)
    })
}
return g.Wait()
```

Это достаточный вариант для многих конечных наборов задач, но не универсальная
замена очереди с приоритетами, состоянием работников или специальным draining.
