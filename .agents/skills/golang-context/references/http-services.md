# Контекст HTTP-запросов и вызовов сервисов

## Контекст входящего запроса

Используй `r.Context()`: он отменяется при разрыве соединения клиентом
или завершении обработчика. Не подменяй его `Background()` внутри той же операции.

```go
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context() // this context is cancelled if the client disconnects

    order, err := h.orderService.Get(ctx, r.PathValue("id"))
    if err != nil {
        if ctx.Err() != nil {
            // Client disconnected, no point writing a response
            return
        }
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(order)
}
```

При использовании в приложении обработай ошибку `Encode`: после начала записи
ответа нельзя надёжно заменить его обычным `http.Error`.

## Middleware с метаданными

Добавляй значения запроса до вызова обработчика. Неэкспортируемый тип ключа
защищает от совпадения ключей разных пакетов.

```go
// Helpers for trace propagation
type contextKey string
const (
    traceIDKey contextKey = "trace_id"
    spanIDKey  contextKey = "span_id"
)

func TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        traceID := r.Header.Get("X-Trace-ID")
        if traceID == "" {
            traceID = generateTraceID()
        }
        spanID := r.Header.Get("X-Span-ID")
        if spanID == "" {
            spanID = generateSpanID()
        }

        ctx := context.WithValue(r.Context(), traceIDKey, traceID)
        ctx = context.WithValue(ctx, spanIDKey, spanID)

        w.Header().Set("X-Trace-ID", traceID)
        w.Header().Set("X-Span-ID", spanID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Propagate trace context to downstream services
func (c *HTTPClient) Do(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, method, url, body)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }

    if traceID, ok := ctx.Value(traceIDKey).(string); ok {
        req.Header.Set("X-Trace-ID", traceID)
    }
    if spanID, ok := ctx.Value(spanIDKey).(string); ok {
        req.Header.Set("X-Span-ID", spanID)
    }
    return c.client.Do(req)
}
```

`X-Trace-ID` и `X-Span-ID` здесь показывают ручную передачу метаданных.
Они не заменяют стандартный протокол трассировки и проверку входящих заголовков.

## Вызовы других сервисов

Передавай контекст в HTTP-клиенты и методы БД с поддержкой контекста:
`http.NewRequestWithContext`, `QueryContext`, `ExecContext`, `QueryRowContext`.
Отмена сможет распространиться на операции, которые её поддерживают.

```go
// ✗ Bad — downstream calls ignore the request context
func (c *PaymentClient) Charge(ctx context.Context, amount int) error {
    req, _ := http.NewRequest("POST", c.url+"/charge", body)
    return c.client.Do(req) // not context-aware
}

// ✓ Good — all downstream operations respect the context
func (c *PaymentClient) Charge(ctx context.Context, amount int) error {
    req, err := http.NewRequestWithContext(ctx, "POST", c.url+"/charge", body)
    if err != nil {
        return fmt.Errorf("creating request: %w", err)
    }
    return c.client.Do(req)
}
```

В примере `Charge` возвращаемые значения HTTP-клиента нужно обработать явно:
`http.Client.Do` возвращает `(*http.Response, error)`, а метод `Charge` — только
`error`. Успешный ответ требует закрытия тела и проверки статуса по контракту API.
Ниже показана отдельно разница между обычным запросом БД и запросом с контекстом.

```go
// ✗ Bad — downstream calls ignore the request context
func (r *UserRepo) FindByID(ctx context.Context, id string) (*User, error) {
    row := r.db.QueryRow("SELECT * FROM users WHERE id = $1", id)
    // ...
}

// ✓ Good — all downstream operations respect the context
func (r *UserRepo) FindByID(ctx context.Context, id string) (*User, error) {
    row := r.db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = $1", id)
    // ...
}
```
