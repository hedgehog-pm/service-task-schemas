# Обработка ошибок и логирование

## Обработай один раз

Обычно либо верни ошибку наверх, либо окончательно обработай её здесь.
Логирование на каждом промежуточном уровне создаёт повторные события.

```go
// ✗ Bad — logs AND returns (duplicate noise)
func processOrder(id string) error {
    err := chargeCard(id)
    if err != nil {
        log.Printf("failed to charge card: %v", err)
        return fmt.Errorf("charging card: %w", err)
    }
    return nil
}

// ✓ Good — return with context, let the caller decide
func processOrder(id string) error {
    err := chargeCard(id)
    if err != nil {
        return oops.
            With("order_id", id).
            Wrapf(err, "charging card")
    }
    return nil
}

// ✓ Good — handle at the top level (HTTP handler, main, etc.)
func handleOrder(w http.ResponseWriter, r *http.Request) {
    err := processOrder(r.FormValue("id"))
    if err != nil {
        slog.Error("order failed", "error", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

Если ошибка подавляется и выполнение продолжается, текущий уровень может быть
окончательным обработчиком. Контракт определяет результат, а не запрет на любые
сочетания логирования и возврата вне контекста.

## Panic и recover

### Когда уместен panic

Нарушенный инвариант или программная ошибка могут означать невозможность
продолжения. Отсутствующий файл, сетевой таймаут и неверный пользовательский
ввод обычно возвращаются как `error`.

```go
// ✓ Acceptable — programmer error in initialization
func MustCompileRegex(pattern string) *regexp.Regexp {
    re, err := regexp.Compile(pattern)
    if err != nil {
        panic(fmt.Sprintf("invalid regex %q: %v", pattern, err))
    }
    return re
}

// ✗ Bad — panic for a normal failure
func GetUser(id string) *User {
    user, err := db.Find(id)
    if err != nil {
        panic(err) // callers cannot recover gracefully
    }
    return user
}
```

### Изоляция panic

`recover` работает в отложенной функции той же горутины.
Ставь его на осмысленной границе изоляции, если безопасное продолжение определено.
Не добавляй recovery автоматически к каждой горутине и не превращай сбой в успех.

```go
func safeHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if r := recover(); r != nil {
                slog.Error("panic recovered",
                    "panic", r,
                    "stack", string(debug.Stack()),
                )
                http.Error(w, "internal error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

## Дополнительная библиотека структурированных ошибок

Следующий пример показывает `samber/oops`: стек, поля пользователя и заказа,
домен, код ошибки и разделение диагностического и публичного сообщения.
Это вариант для проекта, которому такой механизм нужен; не устанавливай
библиотеку вместо стандартных ошибок без отдельного основания.

```go
// ✗ Before — standard errors, no context
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderReq) error {
    err := s.db.Insert(ctx, req.Order)
    if err != nil {
        return fmt.Errorf("inserting order: %w", err)
    }
    return nil
}

// ✓ After — samber/oops, rich context for debugging
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderReq) error {
    err := s.db.Insert(ctx, req.Order)
    if err != nil {
        return oops.
            In("order-service").
            Code("order_insert_failed").
            User(req.UserID).
            With("order_id", req.Order.ID).
            Wrapf(err, "inserting order")
    }
    return nil
}
```

Проверь интеграцию с логгером: наличие полей внутри ошибки ещё не гарантирует,
что используемый handler выведет их в нужном формате.

## slog

Используй существующую настройку проекта. Сообщение события постоянно,
причина передаётся как `slog.Any("error", err)`, контекст — через Context-вызов
при его наличии. Дополнительная библиотека наблюдаемости для этого не требуется.
