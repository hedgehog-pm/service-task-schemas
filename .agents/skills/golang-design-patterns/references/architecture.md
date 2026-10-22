# Выбор архитектуры

## Достаточная структура

Сложность архитектуры определяется задачей, зависимостями и инвариантами,
а не числом строк. Для небольшого инструмента достаточно функции или нескольких
файлов; сервис может выделять обработчики, операции и адаптеры по мере необходимости.
Clean Architecture, hexagonal и DDD применяй при реальной потребности в их границах.
Не спрашивай предпочтение заново, если структура уже определена правилами проекта.

## Предметная логика

Отделяй расчёты и предметные инварианты от запросов к БД, HTTP и очередям,
когда это позволяет понимать правило по входам и результату.

```go
// domain/order.go — pure business logic, no imports from infrastructure
package domain

type Order struct {
    ID     string
    Items  []Item
    Status OrderStatus
}

func (o *Order) AddItem(item Item) error {
    if o.Status != StatusDraft {
        return ErrOrderNotEditable
    }
    o.Items = append(o.Items, item)
    return nil
}
```

Адаптер может зависеть от предметных типов; обратная зависимость на детали
инфраструктуры не должна попадать в сам расчёт.

## Валидация на границах

Проверяй входы HTTP, CLI и сообщений до выполнения операции.
Внутри не повторяй доказанную проверку на каждом слое. Но если тип может
создаваться из нескольких независимых источников, сохраняй его инвариант
на соответствующей общей границе.

```go
// Handler layer — validate here
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
    var req CreateOrderRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid JSON", http.StatusBadRequest)
        return
    }
    if req.UserID == "" {
        http.Error(w, "user_id is required", http.StatusBadRequest)
        return
    }
    if len(req.Items) == 0 {
        http.Error(w, "at least one item required", http.StatusBadRequest)
        return
    }

    // Domain layer trusts this data is valid
    order, err := h.service.CreateOrder(r.Context(), req.UserID, req.Items)
    // ...
}
```

## Недопустимые состояния

Используй типы, когда они выражают реальные ограничения данных.

```go
// Bad — status is a raw string, anything goes
type Order struct {
    Status string // "pending"? "PENDING"? "active"? anything?
}

// Good — typed enum constrains the values
type OrderStatus int

const (
    OrderStatusUnknown   OrderStatus = iota // 0 = invalid
    OrderStatusDraft                        // 1
    OrderStatusConfirmed                    // 2
    OrderStatusShipped                      // 3
)

type Order struct {
    Status OrderStatus
}
```

Не создавай отдельный тип ради формальности; конструктор и закрытые поля
должны действительно обеспечивать заявленный инвариант.

```go
// Bad — email is a raw string, could be anything
func SendEmail(to string, body string) error { ... }

// Good — validated type enforces the constraint
type Email struct {
    address string // unexported: can only be created via constructor
}

func NewEmail(raw string) (Email, error) {
    if !isValidEmail(raw) {
        return Email{}, fmt.Errorf("invalid email: %s", raw)
    }
    return Email{address: raw}, nil
}
```

## Подробные варианты

- [DDD](ddd.md): объекты-значения, агрегаты, ограниченные контексты.
- [Clean Architecture](clean-architecture.md): варианты использования и направление зависимостей.
- [Hexagonal Architecture](hexagonal-architecture.md): порты и адаптеры.

Иллюстративные каталоги в примерах не заменяют структуру проекта из `architecture.md`.
Принципы 12-factor учитывай там, где они относятся к поставке сервиса;
не вводи новый слой или зависимость только ради названия подхода.

## Явные зависимости

Чтение конфигурации, подключение к окружению и запуск работы должны быть видны
при сборке программы. Не прячь существенный I/O в глобальном состоянии.

```go
// Bad — implicit behavior hidden in struct tags and reflection
type Config struct {
    Port int `default:"8080"`
}

// Good — explicit defaults visible in code
func NewConfig() Config {
    return Config{Port: 8080}
}
```

Предпочитай явный конструктор скрытому подключению:

```go
// Bad — implicit dependency via global
func HandleRequest(w http.ResponseWriter, r *http.Request) {
    user := globalDB.FindUser(r.Context(), userID) // where does globalDB come from?
}

// Good — explicit dependency via injection
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
    user := h.db.FindUser(r.Context(), userID) // clear: db is a field on Handler
}
```
