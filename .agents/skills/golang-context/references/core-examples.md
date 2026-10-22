# Примеры основных приёмов

Пример сохраняет контекст запроса на всём пути внешнего вызова.
Типы и функции приложения здесь условные; не создавай их ради воспроизведения фрагмента.

## Передача контекста

```go
// ✗ Bad — creates a new context, breaking the chain
func (s *OrderService) Create(ctx context.Context, order Order) error {
    return s.db.ExecContext(context.Background(), "INSERT INTO orders ...", order.ID)
}

// ✓ Good — propagates the caller's context
func (s *OrderService) Create(ctx context.Context, order Order) error {
    return s.db.ExecContext(ctx, "INSERT INTO orders ...", order.ID)
}
```
