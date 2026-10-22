# Оборачивание и распознавание ошибок

## Оборачивание через %w

`%w` сохраняет причину для `errors.Is` и `errors.As`.
Добавляй контекст там, где текущий уровень знает полезное действие, объект или этап.
Не оборачивай автоматически на каждом слое.

```go
// ✓ Good — wraps with context, preserves the chain
func (s *UserService) GetUser(id string) (*User, error) {
    user, err := s.repo.FindByID(id)
    if err != nil {
        return nil, fmt.Errorf("getting user %s: %w", id, err)
    }
    return user, nil
}
```

### %w и %v на границе API

`%w` открывает причину программному анализу; `%v` сохраняет только её текст.
Решай, должна ли ошибка драйвера становиться частью публичного контракта,
а не скрывай все причины на любой внешней границе механически.

```go
// Internal layer — wrap to preserve chain
func (r *repo) fetch(id string) error {
    return fmt.Errorf("querying database: %w", err)
}

// Public API boundary — break chain to hide internals
func (s *PublicService) GetItem(id string) error {
    err := s.repo.fetch(id)
    if err != nil {
        return fmt.Errorf("item unavailable: %v", err) // %v — callers cannot unwrap
    }
    return nil
}
```

Пример показывает намеренный разрыв цепочки. Если нужно распознаваемое
прикладное состояние, лучше преобразуй известную причину в собственную ошибку.

## Распознавание

### errors.Is

Находит узнаваемое состояние в цепочке.

```go
// ✗ Bad — direct comparison breaks on wrapped errors
if err == sql.ErrNoRows {

// ✓ Good — traverses the entire error chain
if errors.Is(err, sql.ErrNoRows) {
    return nil, ErrNotFound
}
```

### errors.As и errors.AsType

Извлекают ошибку нужного типа из цепочки. Доступность `errors.AsType`
проверяй по версии Go проекта.

```go
// ✗ Bad — type assertion breaks on wrapped errors
if ve, ok := err.(*ValidationError); ok {

// ✓ Good — traverses the entire error chain
var ve *ValidationError
if errors.As(err, &ve) {
    log.Printf("validation failed on field %s: %s", ve.Field, ve.Msg)
}

// ✓ Better (Go 1.26+) — same behavior, simpler syntax
if ve, ok := errors.AsType[*ValidationError](err); ok {
    log.Printf("validation failed on field %s: %s", ve.Field, ve.Msg)
}
```

## errors.Join: Go 1.20+

Объединяет независимые существенные ошибки; каждая остаётся доступна
для `errors.Is` и `errors.As`.

### Несколько ошибок валидации

```go
func validateUser(u User) error {
    var errs []error

    if u.Name == "" {
        errs = append(errs, errors.New("name is required"))
    }
    if u.Email == "" {
        errs = append(errs, errors.New("email is required"))
    }

    return errors.Join(errs...) // returns nil if errs is empty
}
```

### Независимые операции

```go
func closeAll(closers ...io.Closer) error {
    var errs []error
    for _, c := range closers {
        if err := c.Close(); err != nil {
            errs = append(errs, err)
        }
    }
    return errors.Join(errs...)
}
```

### Распознавание внутри объединения

```go
err := errors.Join(ErrNotFound, ErrUnauthorized)

errors.Is(err, ErrNotFound)    // true
errors.Is(err, ErrUnauthorized) // true
```
