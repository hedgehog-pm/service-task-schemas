# Корректная работа с nil

## Методы указателя

Метод на nil-указателе не обязательно вызывает panic: это зависит от обращения
к получателю. Проверяй контракт типа, а не только наличие указателя.

```go
type Logger struct {
    prefix string
}

// ✓ Safe on nil but NEVER do that — does not dereference l
func (l *Logger) IsEnabled() bool {
    return l != nil
}

// ✗ Panics on nil — dereferences l to access prefix
func (l *Logger) Log(msg string) {
    fmt.Printf("[%s] %s\n", l.prefix, msg)
}

var l *Logger
l.IsEnabled() // false — works fine
l.Log("test") // panic: nil pointer dereference
```

### Nil как допустимый получатель

Если nil означает отсутствие необязательного компонента, метод может явно
поддерживать это состояние. Не скрывай таким приёмом ошибки обязательной инициализации.

```go
func (l *Logger) Log(msg string) {
    if l == nil {
        return // silently skip if no logger configured
    }
    fmt.Printf("[%s] %s\n", l.prefix, msg)
}
```

## Nil-функции

Вызов nil-функции вызывает panic. Проверь необязательный callback перед вызовом.

```go
// ✗ Bad — panics if callback was never set
type Worker struct {
    onComplete func(result string)
}

func (w *Worker) Finish(result string) {
    w.onComplete(result) // panic if onComplete is nil
}

// ✓ Good — check before calling
func (w *Worker) Finish(result string) {
    if w.onComplete != nil {
        w.onComplete(result)
    }
}
```

### Функция по умолчанию

Если отсутствие callback означает отсутствие действия, допустим пустой обработчик.
Functional options в этом примере показывают уже существующую форму API,
а не требуют вводить её ради одного callback.

```go
func NewWorker(opts ...Option) *Worker {
    w := &Worker{
        onComplete: func(string) {}, // no-op default
    }
    for _, opt := range opts {
        opt(w)
    }
    return w
}
```

## Nil и error

### Возвращай nil-интерфейс явно

Интерфейс с динамическим типом `*ValidationError` и nil-значением не равен nil.

```go
// ✗ Bad — returns non-nil error interface
func validate(s string) error {
    var err *ValidationError // typed nil
    if s == "" {
        err = &ValidationError{Field: "name"}
    }
    return err // even when err is nil, interface is non-nil
}

// ✓ Good — return nil explicitly
func validate(s string) error {
    if s == "" {
        return &ValidationError{Field: "name"}
    }
    return nil
}
```

### errors.Is и nil

`errors.Is(err, nil)` истинно только для nil-интерфейса и не устраняет проблему typed nil.

## Nil в generic-коде

### comparable и нулевое значение

`comparable` разрешает сравнение значений типа, но не гарантирует, что тип
допускает nil. В первом примере `IsZero` корректно проверяет нулевое значение;
он не является универсальной проверкой nil. Второй пример сужает допустимые типы.

```go
// ✗ Confusing — T may or may not be nillable
func IsZero[T comparable](v T) bool {
    var zero T
    return v == zero // works, but "zero" for *Foo is nil, for int is 0
}

// ✓ Better — be explicit about what "empty" means
func IsNil[T interface{ ~*U }, U any](v T) bool {
    return v == nil
}
```

### Неограниченный параметр типа

Параметр `T any` нельзя непосредственно сравнить с nil. Уточни контракт,
например принимай `*T`; reflection оправдан только реальной необходимостью.

```go
// ✗ Does not compile
func Check[T any](v T) bool {
    return v == nil // compile error: cannot compare T with nil
}

// ✓ Good — use reflect or constrain to pointer types
func IsNilPtr[T any](v *T) bool {
    return v == nil
}
```

## API с инвариантами инициализации

### Конструктор со значениями по умолчанию

Конструктор устанавливает поля для корректной работы клиента.
Он не запрещает внешний литерал `Client{}`, если сам тип экспортируется:
поддержку нулевого значения нужно определять отдельно.

```go
type Client struct {
    httpClient *http.Client
    baseURL    string
}

// Constructor guarantees non-nil fields
func NewClient(baseURL string) *Client {
    return &Client{
        httpClient: http.DefaultClient,
        baseURL:    baseURL,
    }
}
```

### Ленивое создание map

Этот кэш поддерживает нулевое значение: чтение пустого кэша безопасно,
map создаётся при первой записи под mutex.

```go
type Cache struct {
    mu   sync.Mutex
    data map[string]any
}

func (c *Cache) Get(key string) (any, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.data == nil {
        return nil, false
    }
    v, ok := c.data[key]
    return v, ok
}

func (c *Cache) Set(key string, val any) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.data == nil {
        c.data = make(map[string]any)
    }
    c.data[key] = val
}
```

Ошибки с typed nil также разобраны в скилле `golang-error-handling`.
