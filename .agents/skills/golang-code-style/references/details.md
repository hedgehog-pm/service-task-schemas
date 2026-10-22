# Условия и передача аргументов

## Именованные условия

Если имя делает составное условие понятнее, выдели проверки в именованные bool.
Это приём читаемости, а не обязательное преобразование после заданного числа операндов.

```go
// Good — self-documenting
isAdmin := user.Role == RoleAdmin
isOwner := resource.OwnerID == user.ID
hasOverride := permissions.Contains(PermOverride)
if isAdmin || isOwner || hasOverride {
    allow()
}

// Bad — wall of logic
if user.Role == RoleAdmin || resource.OwnerID == user.ID || permissions.Contains(PermOverride) {
    allow()
}
```

Дорогую проверку оставляй внутри условия, чтобы сохранить короткое замыкание.
Вынесенный заранее вызов выполняется даже тогда, когда результат уже известен.

```go
// Good — avoid expensive operation when possible
if isAdmin || isOwner || expensivePermissionCheck(user, resource) {
    allow()
}

// Wasteful — always runs expensive check
canOverride := expensivePermissionCheck(user, resource)
if isAdmin || isOwner || canOverride {
    allow()
}
```

## Значение или указатель

Здесь речь о параметрах функций, а не получателях методов.
Небольшие значения обычно передаются по значению; строка уже содержит указатель и длину.

```go
// Good — value types by value
func FormatUser(name string, age int, createdAt time.Time) string

// Good — pointer for mutation
func PopulateDefaults(cfg *Config)

// Good — pointer when nil is meaningful (optional field update)
func UpdateUser(ctx context.Context, id string, name *string) error

// Bad — pointer for no reason
func Greet(name *string) string
```

Указатель нужен, если функция меняет исходное значение, `nil` имеет смысл
необязательного параметра или копирование большой структуры действительно существенно.
Для `string`, `int`, `bool`, `float64`, `time.Time` и небольших структур,
доступных только для чтения, начни с передачи значения.

При выборе ради производительности учитывай стоимость копирования, косвенного
доступа и локальность памяти. Размер около 128 байт — лишь ориентир из примеров,
а не граница языка или гарантия ускорения. Размещение на стеке и escape analysis
зависят от использования. Измеряй существенный для задачи случай.
