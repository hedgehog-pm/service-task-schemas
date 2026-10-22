# Примеры основных приёмов

Пары примеров показывают риск и его устранение. Точное сравнение float может
быть корректным по контракту; epsilon выбирается по масштабу задачи. Защитная
копия нужна при защите внутреннего состояния, а не для каждого публичного результата.
Нулевое значение поддерживай, когда это естественно для типа. В примере среза
ограничение ёмкости влияет на последующий непустой append, но не копирует массив сразу.

Пример `DB.connection` ниже содержит реальную проблему: ошибка `sql.Open`
отброшена, а `sync.Once` запрещает повторную попытку. Это иллюстрация механизма,
не готовая реализация подключения. В рабочем коде сохрани и верни ошибку,
определи политику повтора и владельца закрытия либо создай БД явно при запуске.

## Typed nil внутри интерфейса

```go
// ✗ Dangerous — interface{type: *MyHandler, value: nil} is not == nil
func getHandler() http.Handler {
    var h *MyHandler // nil pointer
    if !enabled {
        return h // interface{type: *MyHandler, value: nil} != nil
    }
    return h
}

// ✓ Good — return nil explicitly
func getHandler() http.Handler {
    if !enabled {
        return nil // interface{type: nil, value: nil} == nil
    }
    return &MyHandler{}
}
```

## Nil-коллекции

```go
// ✗ Bad — nil map panics on write
var m map[string]int
m["key"] = 1

// ✓ Good — initialize or lazy-init in methods
m := make(map[string]int)

func (r *Registry) Add(name string, val int) {
    if r.items == nil { r.items = make(map[string]int) }
    r.items[name] = val
}
```

## Общий массив и append

```go
// ✗ Dangerous — a and b share backing array
a := make([]int, 3, 5)
b := append(a, 4)
b[0] = 99 // also modifies a[0]

// ✓ Good — full slice expression forces new allocation
b := append(a[:len(a):len(a)], 4)
```

## Проверка перед числовым преобразованием

```go
// ✗ Bad — silently wraps around if val > math.MaxInt32 (3B becomes -1.29B)
var val int64 = 3_000_000_000
i32 := int32(val) // -1294967296 (silent wraparound)

// ✓ Good — check before converting
if val > math.MaxInt32 || val < math.MinInt32 {
    return fmt.Errorf("value %d overflows int32", val)
}
i32 := int32(val)
```

## Сравнение float

```go
// ✗ Bad — floating point arithmetic is not exact
var a, b, c float64 = 0.1, 0.2, 0.3
a+b == c // false

// ✓ Good — use epsilon comparison
const epsilon = 1e-9
math.Abs((a+b)-c) < epsilon // true
```

## Деление на ноль

```go
func avg(total, count int) (int, error) {
    if count == 0 {
        return 0, errors.New("division by zero")
    }
    return total / count, nil
}
```

## Время выполнения defer в цикле

```go
// ✗ Bad — all files stay open until function returns
for _, path := range paths {
    f, _ := os.Open(path)
    defer f.Close() // deferred until function exits
    process(f)
}

// ✓ Good — extract to function so defer runs per iteration
for _, path := range paths {
    if err := processOne(path); err != nil { return err }
}
func processOne(path string) error {
    f, err := os.Open(path)
    if err != nil { return err }
    defer f.Close()
    return process(f)
}
```

## Защита внутреннего состояния

```go
// ✗ Bad — exported slice field, anyone can mutate
type Config struct {
    Hosts []string
}

// ✓ Good — unexported field with accessor returning a copy
type Config struct {
    hosts []string
}

func (c *Config) Hosts() []string {
    return slices.Clone(c.hosts)
}
```

## Нулевое значение

```go
var mu sync.Mutex   // ✓ usable at zero value
var buf bytes.Buffer // ✓ usable at zero value

// ✗ Bad — nil map panics on write
type Cache struct { data map[string]any }
```

## Однократная инициализация

```go
type DB struct {
    once sync.Once
    conn *sql.DB
}

func (db *DB) connection() *sql.DB {
    db.once.Do(func() {
        db.conn, _ = sql.Open("postgres", connStr)
    })
    return db.conn
}
```

## Проверка типа при reflection

```go
v := reflect.ValueOf(x)
if s, ok := reflect.TypeAssert[string](v); ok {
    use(s)
}
```
