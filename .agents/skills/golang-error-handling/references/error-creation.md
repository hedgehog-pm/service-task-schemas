# Создание ошибок

## Ошибки как значения

Ошибка — обычное значение, реализующее интерфейс `error`.

```go
type error interface {
    Error() string
}
```

Функция возвращает ошибку, а вызывающий код обрабатывает её; это не механизм исключений.

```go
// ✗ Bad — silently discarding errors
data, _ := os.ReadFile("config.yaml")

// ✗ Bad — only checking in some branches
result, err := doSomething()
fmt.Println(result) // using result without checking err

// ✓ Good — always check before using other return values
data, err := os.ReadFile("config.yaml")
if err != nil {
    return fmt.Errorf("reading config: %w", err)
}
```

## Текст ошибки

Обычно используй нижний регистр без конечной точки. Не дублируй контекст,
который уже добавляет вызывающий уровень.

```go
// ✗ Bad — capitalized, punctuation, redundant prefix
return errors.New("Failed to connect to database.")
return fmt.Errorf("UserService: failed to fetch user: %w", err)

// ✓ Good — lowercase, no punctuation, concise
return errors.New("connection refused")
return fmt.Errorf("fetching user: %w", err)
```

Полезные префиксы разных уровней образуют понятную цепочку:

```
creating order: charging card: connecting to payment gateway: connection refused
```

Наличие слоя само по себе не обязывает добавлять ещё один префикс.

## errors.New

Для постоянного сообщения или узнаваемой sentinel-ошибки:

```go
var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")
```

## fmt.Errorf

Для сообщения с параметрами и для оборачивания причины:

```go
import "github.com/samber/oops"

// ✗ Avoid at log/APM boundaries — each user/tenant combo becomes a unique group
return fmt.Errorf("user %s not found in tenant %s", userID, tenantID)

// ✓ Prefer for grouped production errors — static message, variable data as structured attributes
return oops.With("user_id", userID).With("tenant_id", tenantID).Errorf("user not found")
```

## Выбор формы

| Потребность | Решение |
| --- | --- |
| Узнать конкретное состояние | Sentinel и `errors.Is` |
| Извлечь поля ошибки | Собственный тип и `errors.As` |
| Сообщить только диагностическую информацию | `errors.New` или `fmt.Errorf` |
| Стек и структурированный контекст | Уже принятый механизм проекта; новая библиотека требует отдельной причины |

## Группировка событий

APM и агрегаторы могут группировать события по сообщению или fingerprint.
Не помещай переменные ID и пути в постоянное сообщение логирования.

```go
import "github.com/samber/oops"

// ✗ Bad at the log boundary — each file/line combo can create a unique group
fmt.Errorf("error in %s at line %d of the csv", csvPath, line)

// ✓ Good (stdlib) — static error, structured attributes at the log site
err := errors.New("csv parsing error")
// ... later, at the logging boundary:
slog.Error("csv parsing failed", "error", err, "csv_file_path", csvPath, "csv_file_line", line)

// ✓ Good (samber/oops, external dependency) — attributes travel with the error
oops.With("csv_file_path", csvPath).With("csv_file_line", line).Errorf("csv parsing error")
```

Динамические сведения в возвращаемой ошибке полезны для диагностики и CLI.
При логировании оставляй текст события постоянным, а ID, пути и саму ошибку
передавай атрибутами. В этом проекте для причины используется `slog.Any("error", err)`.
Статический префикс `fmt.Errorf("fetching user: %w", err)` допустим;
`error` и сообщение события не обязаны иметь одну форму.
Библиотека структурированных ошибок не является обязательным решением.

## Тип ошибки с данными

Создавай отдельный тип, когда вызывающий код использует его поля для решения.

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

// Usage
func validateAge(age int) error {
    if age < 0 {
        return &ValidationError{Field: "age", Message: "must be non-negative"}
    }
    return nil
}
```

### Unwrap

Если тип открывает вложенную причину как часть контракта, реализуй `Unwrap`.

```go
type QueryError struct {
    Query string
    Err   error
}

func (e *QueryError) Error() string {
    return fmt.Sprintf("query %q: %v", e.Query, e.Err)
}

func (e *QueryError) Unwrap() error {
    return e.Err
}
```
