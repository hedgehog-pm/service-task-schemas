# Каналы и select

## Жизненный цикл горутины

До запуска определи, как работа остановится и кто дождётся её завершения.

```go
// ✗ Bad — fire-and-forget, no way to stop or wait
func startWorker() {
    go func() {
        for {
            doWork() // runs forever, leaks on shutdown
        }
    }()
}

// ✓ Good — goroutine respects context cancellation, caller can wait
func startWorker(ctx context.Context) *sync.WaitGroup {
    var wg sync.WaitGroup
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {
            select {
            case <-ctx.Done():
                return
            default:
                doWork(ctx)
            }
        }
    }()
    return &wg
}
```

### Recover на границе горутины

Необработанный panic в горутине завершает процесс. `recover` может изолировать
сбой только в той же горутине и только через отложенную функцию.
Применяй его там, где безопасное продолжение действительно определено контрактом,
а не автоматически в каждой производственной горутине.

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            // ...
        }
    }()
    doWork(ctx)
}()
```

## Направление канала

В сигнатуре укажи, какие операции разрешены вызываемому коду.

```go
// ✗ Bad — caller could accidentally close or send on a receive-only channel
func consume(ch chan int) { ... }

// ✓ Good — compiler enforces correct usage
func produce(ch chan<- int) { ... } // send-only
func consume(ch <-chan int) { ... } // receive-only
```

## Закрытие

Канал закрывает отправитель или координатор, который гарантирует отсутствие
дальнейших отправок. Получатель не должен закрывать канал, в который ещё пишут.

```go
// ✓ Good — producer closes when done
func generate(ctx context.Context) <-chan int {
    ch := make(chan int)
    go func() {
        defer close(ch) // sender closes
        for i := 0; ; i++ {
            select {
            case ch <- i:
            case <-ctx.Done():
                return
            }
        }
    }()
    return ch
}
```

## Размер буфера

| Размер | Когда подходит |
| --- | --- |
| 0 | Нужна синхронная передача отправитель–получатель |
| 1 | Одноразовый результат или один ожидающий сигнал |
| N | Ограниченная очередь с известным поведением при заполнении |

Размер определяется протоколом и нагрузкой; ни нулевой буфер, ни измерение
производительности не обязательны для каждого канала.

```go
// ✓ Good — unbuffered for synchronous handoff
ch := make(chan Result)

// ✓ Good — buffered 1 for signal
done := make(chan struct{}, 1)

// ✗ Suspicious — arbitrary large buffer hides backpressure problems
// Give explanation in comments.
ch := make(chan Task, 1000) // why 1000? what if it fills?
```

## Неблокирующее взаимодействие

`select` позволяет ожидать несколько событий или выполнить запасную ветку
`default`. Для ожидания, которое должно отменяться, добавь `ctx.Done()`
или другой гарантированный путь завершения. У `select` нет приоритета отмены.

```go
func process(ctx context.Context, in <-chan Task, out chan<- Result) {
    for {
        select {
        case <-ctx.Done():
            return
        case task, ok := <-in:
            if !ok {
                return // channel closed
            }
            result := handle(ctx, task)
            select {
            case out <- result:
            case <-ctx.Done():
                return
            }
        }
    }
}
```

## Таймеры в горячем цикле

Повторный `time.After` создаёт таймеры. Для часто повторяемого ожидания
рассмотри переиспользуемый `time.Timer`.

```go
// ✗ Bad — creates a new timer on every iteration
for {
    select {
    case msg := <-ch:
        handle(msg)
    case <-time.After(5 * time.Second): // repeated allocation/churn
        handleTimeout()
    }
}

// ✓ Good (Go 1.23+) — reuse the timer
timer := time.NewTimer(5 * time.Second)
defer timer.Stop()
for {
    select {
    case msg := <-ch:
        timer.Stop()
        timer.Reset(5 * time.Second)
        handle(msg)
    case <-timer.C:
        handleTimeout()
        timer.Reset(5 * time.Second)
    }
}
```

Для Go до 1.23 перед `Reset` учитывай возможное старое значение канала и правила
`Stop`/drain; координация с параллельным чтением должна быть явной.
В Go 1.23+ при новых правилах таймеров после `Stop` не читается старое значение.
Проверь версию модуля и настройки совместимости таймеров.
