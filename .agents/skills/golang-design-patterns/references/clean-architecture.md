# Clean Architecture в Go

## Когда подходит

Когда важны независимость предметных правил от инфраструктуры и явное направление
зависимостей. Размер проекта сам по себе не требует слоёв.

## Направление зависимостей

Внутренние слои не импортируют внешние адаптеры.

```
Frameworks & Drivers  →  Interface Adapters  →  Use Cases  →  Entities
(HTTP, DB, gRPC)         (handlers, repos)      (app logic)   (domain)
```

Интерфейс зависимости определяет потребитель; адаптер реализует нужные операции.
Объявляй интерфейс только при реальном использовании этой границы.

## Пример структуры

Это иллюстрация подхода, а не предписание переименовать каталоги проекта.

```
order-service/
├── cmd/
│   └── server/
│       └── main.go                  # Wiring only — builds the dependency graph
├── internal/
│   ├── entity/
│   │   ├── order.go                 # Order entity + business rules
│   │   ├── item.go                  # OrderItem
│   │   └── status.go                # OrderStatus enum
│   ├── order/
│   │   ├── place.go                 # PlaceOrderUseCase
│   │   ├── cancel.go                # CancelOrderUseCase
│   │   └── port.go                  # Interfaces this use case depends on
│   ├── adapter/
│   │   ├── handler/
│   │   │   └── order_handler.go    # HTTP handler — calls use cases
│   │   ├── repository/
│   │   │   └── order_postgres.go   # OrderRepository — implements port
│   │   └── gateway/
│   │       └── payment_client.go   # External payment API client
│   └── infrastructure/
│       ├── router.go               # HTTP router setup
│       ├── database.go             # DB connection
│       └── config.go               # Config loading
├── go.mod
└── go.sum
```

## Предметная сущность

```go
// internal/entity/order.go
package entity

type Order struct {
    ID     string
    Items  []Item
    Status OrderStatus
}

func (o *Order) Cancel() error {
    if o.Status == StatusShipped {
        return ErrCannotCancelShipped
    }
    o.Status = StatusCancelled
    return nil
}

func (o *Order) Total() int64 {
    var sum int64
    for _, item := range o.Items {
        sum += item.Price * int64(item.Quantity)
    }
    return sum
}
```

## Вариант использования

Операция определяет нужные ей зависимости.

```go
// internal/order/port.go
package order

// Ports — interfaces defined by the use case, implemented by adapters
type OrderRepository interface {
    Save(ctx context.Context, order *entity.Order) error
    FindByID(ctx context.Context, id string) (*entity.Order, error)
}

type PaymentGateway interface {
    Charge(ctx context.Context, orderID string, amount int64) error
}
```

Её последовательность остаётся рядом с предметным действием.

```go
// internal/order/place.go
package order

type PlaceOrderUseCase struct {
    orders   OrderRepository
    payments PaymentGateway
}

func NewPlaceOrderUseCase(orders OrderRepository, payments PaymentGateway) *PlaceOrderUseCase {
    return &PlaceOrderUseCase{orders: orders, payments: payments}
}

func (uc *PlaceOrderUseCase) Execute(ctx context.Context, orderID string) error {
    order, err := uc.orders.FindByID(ctx, orderID)
    if err != nil {
        return fmt.Errorf("finding order: %w", err)
    }

    if err := uc.payments.Charge(ctx, order.ID, order.Total()); err != nil {
        return fmt.Errorf("charging payment: %w", err)
    }

    order.Status = entity.StatusPlaced
    return uc.orders.Save(ctx, order)
}
```

## Адаптер

```go
// internal/adapter/repository/order_postgres.go
package repository

type OrderPostgres struct {
    db *sql.DB
}

func NewOrderPostgres(db *sql.DB) *OrderPostgres {
    return &OrderPostgres{db: db}
}

func (r *OrderPostgres) FindByID(ctx context.Context, id string) (*entity.Order, error) {
    // SQL query, scan into entity.Order
}

func (r *OrderPostgres) Save(ctx context.Context, order *entity.Order) error {
    // SQL upsert
}
```

## HTTP-обработчик

```go
// internal/adapter/handler/order_handler.go
package handler

type OrderHandler struct {
    placeOrder *usecase.PlaceOrderUseCase
}

func (h *OrderHandler) HandlePlaceOrder(w http.ResponseWriter, r *http.Request) {
    orderID := chi.URLParam(r, "id")

    if err := h.placeOrder.Execute(r.Context(), orderID); err != nil {
        // Map domain errors to HTTP status codes
        http.Error(w, err.Error(), mapToHTTPStatus(err))
        return
    }

    w.WriteHeader(http.StatusOK)
}
```

## Главное правило

В примере `usecase/order/port.go` принадлежит потребителю `OrderRepository`,
а `adapter/repository/` реализует его без обратной зависимости use case на БД.
Наличие возможной будущей БД не является основанием создавать интерфейс заранее.

## Сборка зависимостей

Зависимости связываются в точке входа. Путь `cmd/server` в схеме иллюстративный;
в проекте сервиса используется `cmd/service`. Для явной сборки контейнер DI не требуется.
