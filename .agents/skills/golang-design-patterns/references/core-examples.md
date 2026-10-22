# Примеры основных приёмов

Functional options, таймауты и структуры здесь — варианты решения конкретной задачи.
Они не требуют менять существующий API или вводить варианты конфигурации заранее.
`log.Fatal` в примере `init` относится к отрицательному примеру скрытого запуска,
а рабочее логирование проекта использует `slog`. Пример regexp демонстрирует
переиспользование скомпилированного шаблона, а не полную валидацию email.

## Functional options как вариант API

```go
type Server struct {
    addr         string
    readTimeout  time.Duration
    writeTimeout time.Duration
    maxConns     int
}

type Option func(*Server)

func WithReadTimeout(d time.Duration) Option {
    return func(s *Server) { s.readTimeout = d }
}

func WithWriteTimeout(d time.Duration) Option {
    return func(s *Server) { s.writeTimeout = d }
}

func WithMaxConns(n int) Option {
    return func(s *Server) { s.maxConns = n }
}

func NewServer(addr string, opts ...Option) *Server {
    // Default options
    s := &Server{
        addr:         addr,
        readTimeout:  5 * time.Second,
        writeTimeout: 10 * time.Second,
        maxConns:     100,
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
srv := NewServer(":8080",
    WithReadTimeout(30*time.Second),
    WithMaxConns(500),
)
```

## Явная инициализация

```go
// Bad — hidden global state
var db *sql.DB

func init() {
    var err error
    db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatal(err)
    }
}

// Good — explicit initialization, injectable
func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}
```

## Нулевое состояние перечисления

```go
type Status int

const (
    StatusUnknown Status = iota // 0 = invalid/unset
    StatusActive                // 1
    StatusInactive              // 2
    StatusSuspended             // 3
)
```

## Переиспользование regexp

```go
// Good — compiled once at package level
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func ValidateEmail(email string) bool {
    return emailRegex.MatchString(email)
}
```

## Встраивание ресурсов

```go
import "embed"

//go:embed templates/*
var templateFS embed.FS

//go:embed version.txt
var version string
```

## Закрытие ресурсов

```go
f, err := os.Open(path)
if err != nil {
    return err
}
defer f.Close() // right here, not 50 lines later

rows, err := db.QueryContext(ctx, query)
if err != nil {
    return err
}
defer rows.Close()
```

## Бюджет внешнего вызова

```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

resp, err := httpClient.Do(req.WithContext(ctx))
```
