# Hexagonal Architecture: порты и адаптеры

## Когда подходит

Когда предметная операция используется через несколько входов или взаимодействует
с несколькими внешними системами и нужна явная изоляция этих взаимодействий.
Не вводи набор портов автоматически для любого CRUD или библиотеки.

## Понятия

- Предметная модель содержит правила и типы.
- Входной порт описывает операции, которыми пользуется внешний вызывающий код.
- Выходной порт описывает операции инфраструктуры, нужные предметной операции.
- Входной адаптер переводит HTTP, gRPC, CLI или сообщение в вызов операции.
- Выходной адаптер реализует работу с БД, платёжным API, кэшем.

Прикладной интерфейс определяй в пакете потребителя; отдельный каталог `ports`
не является обязательным решением для этого проекта.

## Пример структуры

```
order-service/
├── cmd/
│   ├── server/
│   │   └── main.go                  # HTTP server wiring
│   └── worker/
│       └── main.go                  # Message consumer wiring
├── internal/
│   ├── domain/
│   │   ├── order.go                 # Order entity + business rules
│   │   ├── item.go                  # OrderItem
│   │   └── status.go               # OrderStatus enum
│   ├── port/
│   │   ├── incoming.go             # Primary ports (OrderService interface)
│   │   └── outgoing.go             # Secondary ports (OrderRepository, PaymentGateway)
│   ├── service/
│   │   └── order_service.go        # Implements primary ports — orchestrates domain + secondary ports
│   └── adapter/
│       ├── primary/
│       │   ├── http/
│       │   │   ├── router.go
│       │   │   └── order_handler.go # HTTP adapter — calls OrderService
│       │   └── grpc/
│       │       └── order_server.go  # gRPC adapter — calls OrderService
│       └── secondary/
│           ├── postgres/
│           │   └── order_repo.go    # Implements OrderRepository
│           └── stripe/
│               └── payment.go      # Implements PaymentGateway
├── go.mod
└── go.sum
```

## Предметная модель

```go
// internal/domain/order.go
package domain

type Order struct {
    ID     string
    Items  []Item
    Status OrderStatus
}

func (o *Order) Ship() error {
    if o.Status != StatusPaid {
        return ErrOrderNotPaid
    }
    o.Status = StatusShipped
    return nil
}
```

## Порты

Входная операция и внешняя зависимость показаны отдельно.
Создавай такие интерфейсы, только если их границы действительно нужны потребителю.

```go
// internal/port/incoming.go
package port

// Primary port — how the outside world drives the application
type OrderService interface {
    PlaceOrder(ctx context.Context, items []domain.Item) (string, error)
    ShipOrder(ctx context.Context, orderID string) error
    GetOrder(ctx context.Context, orderID string) (*domain.Order, error)
}
```

Внешние операции, от которых зависит сервис:

```go
// internal/port/outgoing.go
package port

// Secondary ports — how the application reaches external systems
type OrderRepository interface {
    Save(ctx context.Context, order *domain.Order) error
    FindByID(ctx context.Context, id string) (*domain.Order, error)
}

type PaymentGateway interface {
    Charge(ctx context.Context, orderID string, amount int64) error
}
```

## Сервис

```go
// internal/service/order_service.go
package service

type orderService struct {
    orders   port.OrderRepository
    payments port.PaymentGateway
}

func NewOrderService(orders port.OrderRepository, payments port.PaymentGateway) port.OrderService {
    return &orderService{orders: orders, payments: payments}
}

func (s *orderService) PlaceOrder(ctx context.Context, items []domain.Item) (string, error) {
    order := domain.NewOrder(items)

    if err := s.payments.Charge(ctx, order.ID, order.Total()); err != nil {
        return "", fmt.Errorf("charging payment: %w", err)
    }

    if err := s.orders.Save(ctx, order); err != nil {
        return "", fmt.Errorf("saving order: %w", err)
    }

    return order.ID, nil
}
```

## HTTP-адаптер

```go
// internal/adapter/primary/http/order_handler.go
package http

type OrderHandler struct {
    svc port.OrderService
}

func NewOrderHandler(svc port.OrderService) *OrderHandler {
    return &OrderHandler{svc: svc}
}

func (h *OrderHandler) HandlePlaceOrder(w http.ResponseWriter, r *http.Request) {
    var req PlaceOrderRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    id, err := h.svc.PlaceOrder(r.Context(), req.Items)
    if err != nil {
        http.Error(w, err.Error(), mapToHTTPStatus(err))
        return
    }

    json.NewEncoder(w).Encode(map[string]string{"id": id})
}
```

## Адаптер хранения

```go
// internal/adapter/secondary/postgres/order_repo.go
package postgres

type OrderRepo struct {
    db *sql.DB
}

func NewOrderRepo(db *sql.DB) *OrderRepo {
    return &OrderRepo{db: db}
}

func (r *OrderRepo) Save(ctx context.Context, order *domain.Order) error {
    // SQL upsert
}

func (r *OrderRepo) FindByID(ctx context.Context, id string) (*domain.Order, error) {
    // SQL query
}
```

## Несколько входов

Одну операцию могут вызывать HTTP, gRPC и обработчик сообщений.
Отдельный `cmd` нужен отдельной программе, а не обязательно каждому адаптеру:
несколько протоколов могут жить в одном процессе по контракту сервиса.

## Сборка

Создай адаптеры и передай их операции в точке входа.
Схема каталогов не заменяет принятые в проекте `cmd/service` и `internal`.
