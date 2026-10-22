# Значения контекста и межсервисная трассировка

## Что хранить в контексте

Передавай метаданные срока жизни запроса: trace ID, span ID, request ID,
подтверждённого пользователя или tenant, correlation ID. Обязательные аргументы,
конфигурацию и инфраструктурные зависимости передавай явно.
Ключ собственного неэкспортируемого типа предотвращает коллизии между пакетами.

```go
// ✓ Good — unexported key type prevents collisions
type contextKey string

const (
    traceIDKey   contextKey = "trace_id"
    requestIDKey contextKey = "request_id"
)

func WithTraceID(ctx context.Context, traceID string) context.Context {
    return context.WithValue(ctx, traceIDKey, traceID)
}

func TraceIDFromContext(ctx context.Context) (string, bool) {
    traceID, ok := ctx.Value(traceIDKey).(string)
    return traceID, ok
}
```

Строковые ключи не обеспечивают такой изоляции:

```go
// ✗ Bad — string keys collide across packages
ctx = context.WithValue(ctx, "trace_id", traceID) // another package could use the same key
```

## Контекст или аргумент функции

| Данные | Место | Причина |
| --- | --- | --- |
| trace_id, span_id, request_id | Контекст | Метаданные запроса для наблюдаемости |
| Подтверждённый пользователь или tenant | Контекст | Метаданные запроса, пересекающие границы API |
| Соединение с БД | Явная зависимость | Инфраструктурный ресурс |
| Feature flags | Явная конфигурация | Настройки поведения |
| ID объекта и данные заказа | Аргументы | Входы бизнес-операции |
| Logger | Зависит от API проекта | В контексте уместен логгер с полями запроса; иначе явная зависимость |

## Передача трассировки между сервисами

Контекст несёт состояние трассировки внутри процесса; при сетевом вызове
его нужно перенести в заголовки, а на входе следующего сервиса — извлечь обратно.

```go
// Middleware injects trace_id from incoming request headers into context
func TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        traceID := r.Header.Get("X-Trace-ID")
        if traceID == "" {
            traceID = generateTraceID()
        }

        ctx := WithTraceID(r.Context(), traceID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// When making outbound HTTP calls, inject trace_id from context into headers
func (c *HTTPClient) Do(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
    req, err := http.NewRequestWithContext(ctx, method, url, body)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }

    // Propagate trace_id to downstream service
    if traceID, ok := TraceIDFromContext(ctx); ok {
        req.Header.Set("X-Trace-ID", traceID)
    }

    return c.client.Do(req)
}
```

При использовании OpenTelemetry эту работу выполняют соответствующая
инструментация и propagator, например `propagation.TraceContext`.
Одной передачи `context.Context` в `http.Client.Do` недостаточно для автоматической
записи заголовков трассировки. Пример с `X-Trace-ID` иллюстрирует механизм,
не предписывая установку собственного протокола или новой библиотеки.
