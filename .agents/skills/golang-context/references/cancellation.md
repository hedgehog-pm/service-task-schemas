# Отмена, таймауты и дедлайны

## Отмена

`context.WithCancel` возвращает дочерний контекст и функцию отмены.
Вызов `cancel()` закрывает `Done()` и сообщает слушателям о завершении.

```go
func processItems(ctx context.Context, items []Item) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel() // always defer cancel to free resources

    errCh := make(chan error, len(items))
    for _, item := range items {
        go func(item Item) {
            errCh <- processOne(ctx, item)
        }(item)
    }

    for range items {
        if err := <-errCh; err != nil {
            cancel() // cancel remaining goroutines on first error
            return fmt.Errorf("processing items: %w", err)
        }
    }
    return nil
}
```

Этот пример показывает распространение отмены. Он запускает по горутине на элемент
и возвращается после первой ошибки, не дожидаясь остальных. Для большого набора
или обязательного ожидания выбери ограниченную группу и явное завершение работников.

### Зачем нужен defer cancel

`WithCancel`, `WithTimeout` и `WithDeadline` создают состояние отмены;
таймаут и дедлайн также используют ресурсы таймера. Вызывай `cancel` на каждом
пути выхода, кроме явной передачи владения контекстом и функцией отмены наружу.
Для работы в пределах функции обычно подходит немедленный `defer cancel()`.

```go
// ✗ Bad — cancel is never called, resources leak
func fetch(ctx context.Context) error {
    ctx, _ = context.WithTimeout(ctx, 5*time.Second)
    return doWork(ctx)
}

// ✓ Good — scoped work, defer cancel immediately
func fetch(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    return doWork(ctx)
}
```

## Таймаут и дедлайн

### WithTimeout: относительная длительность

```go
func (s *UserService) GetUser(ctx context.Context, id string) (*User, error) {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    return s.repo.FindByID(ctx, id)
}
```

### WithDeadline: абсолютное время

```go
func (s *BatchService) ProcessBatch(ctx context.Context, batch Batch) error {
    // The batch must complete by its SLA deadline
    ctx, cancel := context.WithDeadline(ctx, batch.SLADeadline)
    defer cancel()

    for _, item := range batch.Items {
        if err := s.process(ctx, item); err != nil {
            return fmt.Errorf("processing batch item %s: %w", item.ID, err)
        }
    }
    return nil
}
```

### Более ранний дедлайн имеет приоритет

Дочерний контекст с таймаутом 10 секунд не продлевает оставшиеся 2 секунды родителя.
Числа в примерах иллюстрируют механизм; бюджет операции задаётся её контрактом.

```go
// Parent has 2s timeout — child's 10s is effectively ignored
parentCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()

childCtx, childCancel := context.WithTimeout(parentCtx, 10*time.Second)
defer childCancel()
// childCtx expires after 2s, not 10s
```

## Реакция на отмену

### Ожидание через select

Отслеживай `ctx.Done()` рядом с другой ожидаемой работой.

```go
func poll(ctx context.Context, interval time.Duration) error {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err() // context.Canceled or context.DeadlineExceeded
        case <-ticker.C:
            if err := doWork(ctx); err != nil {
                return fmt.Errorf("polling: %w", err)
            }
        }
    }
}
```

### Проверка при вычислениях

Для вычислительного цикла проверяй `ctx.Err()` в подходящих точках.

```go
func processLargeDataset(ctx context.Context, items []Item) error {
    for i, item := range items {
        if ctx.Err() != nil {
            return fmt.Errorf("processing interrupted after %d/%d items: %w", i, len(items), ctx.Err())
        }
        process(item)
    }
    return nil
}
```

## context.AfterFunc: Go 1.21+

Регистрирует обработчик, который после отмены запускается в отдельной горутине.
Возвращённая функция `stop` отменяет ещё не начавшийся обработчик. Она не ждёт
завершения уже запущенного обработчика; при необходимости синхронизируй доступ к ресурсу.

```go
func watchResource(ctx context.Context, res *Resource) {
    stop := context.AfterFunc(ctx, func() {
        // Runs in a new goroutine when ctx is cancelled
        res.Release()
    })

    // If you no longer need the callback, cancel it:
    // stop() returns true if the callback was successfully cancelled
    _ = stop
}
```

## context.WithoutCancel: Go 1.21+

Сохраняет значения родителя, но отделяет отмену и дедлайн; `Done()` возвращает nil.
Подходит для намеренно отдельного срока жизни после завершения запроса.

```go
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    order, err := h.orderService.Create(ctx, req)
    if err != nil {
        // handle error
        return
    }

    // Audit log must complete even if the client disconnects.
    // WithoutCancel preserves context values (trace_id) but detaches cancellation.
    auditCtx := context.WithoutCancel(ctx)
    go h.auditService.LogOrderCreated(auditCtx, order)

    w.WriteHeader(http.StatusCreated)
}
```

`ctx` запроса отменяется при его завершении, а `Background()` теряет его значения.
`WithoutCancel` сохраняет метаданные, но сам по себе не обеспечивает надёжную
доставку аудита, ограничение времени, реакцию на остановку сервиса или ожидание
фоновой горутины. Установи владельца такой работы и необходимые гарантии отдельно.
