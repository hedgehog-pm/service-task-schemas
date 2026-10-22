# Управление ресурсами

## Освобождение после открытия

После проверки ошибки открытия обеспечь закрытие на всех путях выхода,
обычно немедленным `defer`. Ресурс должен действительно иметь метод `Close`:
например, `bufio.Scanner` сам по себе не закрывает обёрнутый источник.

```go
// Good — defer is right next to open
f, err := os.Open(path)
if err != nil {
    return err
}
defer f.Close()

// Bad — Close() is far from Open(), easy to forget when adding early returns
f, err := os.Open(path)
if err != nil {
    return err
}
// ... 50 lines of code ...
f.Close() // might never run if a new return is added above
```

Для записи проверяй ошибки `Flush` и `Close`, если от них зависит успешность операции.
Для чтения различай необходимость освобождения и значимость ошибки закрытия.

```go
resp, err := http.Get(url)
if err != nil {
    return err
}
defer resp.Body.Close()

rows, err := db.QueryContext(ctx, query)
if err != nil {
    return err
}
defer rows.Close()
```

## runtime.AddCleanup: Go 1.24+

При необходимости страховочного cleanup предпочитай `AddCleanup` старому
`SetFinalizer`. Это не замена явному закрытию и не гарантия своевременного выполнения.

```go
type Resource struct {
    handle uintptr
}

func NewResource() *Resource {
    r := &Resource{handle: acquireHandle()}
    runtime.AddCleanup(r, func(handle uintptr) {
        releaseHandle(handle)
    }, r.handle)
    return r
}
```

Можно зарегистрировать несколько cleanup. Аргумент обработчика не должен удерживать
сам объект; это позволяет избежать воскрешения и работать с циклами ссылок.
Следи за временем жизни объекта и при необходимости `runtime.KeepAlive`.

## Ограниченный пул

Канал фиксированной ёмкости может представлять ограниченный набор ресурсов.
`sync.Pool` не является таким пулом: сборщик мусора вправе удалить его элементы,
а управление открытыми соединениями требует явного владельца и закрытия.

```go
type ConnPool struct {
    conns chan *Conn
}

func NewConnPool(maxSize int, factory func() (*Conn, error)) (*ConnPool, error) {
    pool := &ConnPool{
        conns: make(chan *Conn, maxSize),
    }
    // Pre-fill with initial connections
    for range maxSize {
        conn, err := factory()
        if err != nil {
            return nil, fmt.Errorf("creating connection: %w", err)
        }
        pool.conns <- conn
    }
    return pool, nil
}

func (p *ConnPool) Get(ctx context.Context) (*Conn, error) {
    select {
    case conn := <-p.conns:
        return conn, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

func (p *ConnPool) Put(conn *Conn) {
    select {
    case p.conns <- conn:
    default:
        conn.Close() // pool is full, discard
    }
}
```

Если получение ресурса должно отменяться, добавь контекст в ожидание.
При наличии встроенного пула, например у `sql.DB`, используй его возможности
вместо второго самодельного слоя.

## Корректная остановка

`signal.NotifyContext` связывает сигналы процесса с отменой.
Завершение активных запросов выполняется с отдельным ограниченным контекстом,
который ещё не отменён сигналом остановки.

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM,
    )
    defer stop()

    srv := &http.Server{Addr: ":8080", Handler: router}

    // Start server in background
    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            slog.Error("server error", "error", err)
        }
    }()

    slog.Info("server started", "addr", ":8080")

    // Wait for interrupt signal
    <-ctx.Done()
    slog.Info("shutting down...")

    // Give outstanding requests time to complete
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(shutdownCtx); err != nil {
        slog.Error("shutdown error", "error", err)
    }

    // Close other resources: database connections, message queues, etc.
    db.Close()
    slog.Info("shutdown complete")
}
```

Последовательность: получить сигнал, прекратить приём новой работы, дождаться
активной или отменить её по контракту, обработать ошибку остановки, затем закрыть
ресурсы, которые больше не используются. Не каждый канал нужно закрывать:
закрытие определяется протоколом и владельцем отправки.
