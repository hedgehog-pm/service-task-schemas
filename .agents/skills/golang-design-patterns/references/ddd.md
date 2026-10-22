# Domain-Driven Design в Go

## Когда подходит

Когда сложные предметные правила, границы согласованности и язык бизнеса
существенны для устройства программы. Число строк и наличие CRUD сами по себе
не являются достаточным критерием выбора DDD.

## Основные элементы

| Понятие | Представление в Go | Назначение |
| --- | --- | --- |
| Сущность | Структура с идентификатором | Идентичность и жизненный цикл |
| Объект-значение | Значение с инвариантами | Величина или характеристика без собственной идентичности |
| Агрегат | Корневая сущность и её состав | Граница согласованности изменений |
| Репозиторий | Нужный потребителю контракт хранения | Загрузка и сохранение агрегата |
| Предметная операция | Функция или тип | Правило, охватывающее несколько объектов |
| Предметное событие | Значение с фактом | Явное взаимодействие контекстов |

## Структура по ограниченным контекстам

Группировка по предметному контексту может собрать рядом модель, операции и адаптеры.
Это пример организации, а не обязательное дерево каталогов для любого сервиса.

```
order-service/
├── cmd/
│   └── server/
│       └── main.go                  # Wiring only
├── internal/
│   ├── order/                       # Bounded context: Order
│   │   ├── domain/
│   │   │   ├── order.go             # Order aggregate root
│   │   │   ├── item.go              # OrderItem entity
│   │   │   ├── status.go            # OrderStatus enum
│   │   │   ├── repository.go        # OrderRepository interface
│   │   │   └── events.go            # OrderPlaced, OrderShipped events
│   │   ├── application/
│   │   │   ├── place_order.go       # PlaceOrderHandler (command)
│   │   │   └── get_order.go         # GetOrderHandler (query)
│   │   └── adapters/
│   │       ├── persistence/
│   │       │   └── postgres.go      # OrderRepository implementation
│   │       └── http/
│   │           └── handler.go       # HTTP transport
│   ├── billing/                     # Bounded context: Billing (another example)
│   │   ├── domain/
│   │   ├── application/
│   │   └── adapters/
│   ├── shared/
│   │   └── money.go                 # Value object reused across contexts
│   └── events/
│       └── publisher.go             # Shared event bus (infrastructure)
├── go.mod
└── go.sum
```

Контексты имеют явные границы. Общая инфраструктура — например доставка событий —
отделяется от внутренних типов конкретного контекста.
Название `adapters` или `infrastructure` выбирай по существующим соглашениям,
не переименовывай пакет только ради соответствия схеме.

## Объект-значение Money

```go
// internal/domain/shared/money.go
package shared

type Money struct {
    amount   int64  // cents — avoids float precision issues
    currency string
}

func NewMoney(amount int64, currency string) (Money, error) {
    if currency == "" {
        return Money{}, errors.New("currency is required")
    }
    return Money{amount: amount, currency: currency}, nil
}

func (m Money) Add(other Money) (Money, error) {
    if m.currency != other.currency {
        return Money{}, fmt.Errorf("cannot add %s to %s", other.currency, m.currency)
    }
    return Money{amount: m.amount + other.amount, currency: m.currency}, nil
}
```

## Корень агрегата Order

```go
// internal/domain/order/order.go
package order

type Order struct {
    id     string
    items  []Item
    status Status
    total  shared.Money
}

func NewOrder(id string) *Order {
    return &Order{id: id, status: StatusDraft}
}

// All mutations go through the aggregate root
func (o *Order) AddItem(item Item) error {
    if o.status != StatusDraft {
        return ErrOrderNotEditable
    }
    o.items = append(o.items, item)
    return o.recalculateTotal()
}

func (o *Order) Place() (OrderPlaced, error) {
    if len(o.items) == 0 {
        return OrderPlaced{}, ErrEmptyOrder
    }
    o.status = StatusPlaced
    return OrderPlaced{OrderID: o.id, Total: o.total}, nil
}
```

## Контракт хранения

В исходном примере репозиторий находится рядом с доменной моделью.
Для этого проекта действует более точное правило: интерфейс объявляется
в пакете операции, которая его использует, когда он ей действительно нужен.
Если модель только передаётся как данные, ей такой интерфейс не требуется.

```go
// internal/order/domain/repository.go
package domain

type Repository interface {
    Save(ctx context.Context, order *Order) error
    FindByID(ctx context.Context, id string) (*Order, error)
}
```

Адаптер хранения реализует контракт потребителя без обратного импорта адаптера
в предметную модель. Пример не является поводом ввести репозиторий заранее.

## Прикладная операция

```go
// internal/order/application/place_order.go
package application

import (
    "context"
    "fmt"

    "myapp/internal/order/domain"
)

type PlaceOrderHandler struct {
    orders domain.Repository
    events EventPublisher
}

func (h *PlaceOrderHandler) Handle(ctx context.Context, cmd PlaceOrderCommand) error {
    order, err := h.orders.FindByID(ctx, cmd.OrderID)
    if err != nil {
        return fmt.Errorf("finding order: %w", err)
    }

    evt, err := order.Place()
    if err != nil {
        return fmt.Errorf("placing order: %w", err)
    }

    if err := h.orders.Save(ctx, order); err != nil {
        return fmt.Errorf("saving order: %w", err)
    }

    return h.events.Publish(ctx, evt)
}
```

## Ограниченные контексты

Контекст имеет собственные модель и правила. Для обмена используй явный контракт
или перевод внешнего представления в локальные типы.

```go
// internal/billing/adapters/events/order_events.go
package events

import (
    "myapp/internal/events"
    "myapp/internal/billing/domain"
)

type OrderPlacedSubscriber struct {
    invoices domain.InvoiceRepository
}

// Receives order.OrderPlaced, translates to billing domain
func (s *OrderPlacedSubscriber) OnOrderPlaced(evt events.OrderPlaced) error {
    // Translate and create invoice
    return s.invoices.Create(evt.OrderID, evt.Total)
}
```

Смысл такого слоя перевода — не распространять чужую внутреннюю модель по системе.
Для больших независимо поставляемых частей возможны отдельные Go-модули и workspace;
это вариант поставки, а не обязательная часть DDD.

## Сборка зависимостей

Собирай зависимости явными конструкторами в точке входа.
Примеры `cmd/server` не меняют принятую в проекте точку входа `cmd/service`.
