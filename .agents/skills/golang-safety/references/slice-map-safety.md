# Срезы и map

## Захват переменной range

### До Go 1.22

Переменная range переиспользовалась между итерациями. Захват её в замыкании
или сохранение адреса могли связать все обращения с последним значением.

```go
// ✗ Bad (pre-1.22) — all goroutines see the last value of v
var funcs []func()
for _, v := range []string{"a", "b", "c"} {
    funcs = append(funcs, func() { fmt.Println(v) })
}
for _, f := range funcs {
    f() // prints "c", "c", "c"
}

// ✓ Fix (pre-1.22) — shadow the variable
for _, v := range []string{"a", "b", "c"} {
    v := v // re-declare v in inner scope
    funcs = append(funcs, func() { fmt.Println(v) })
}
```

### Начиная с Go 1.22

Переменные, объявленные в цикле через `:=`, создаются для каждой итерации.
Проверь версию языка в `go.mod`: модуль с `go 1.21` сохраняет прежнюю семантику.
Присваивание заранее объявленной переменной через `=` не создаёт новую переменную.

## Адрес переменной цикла

До Go 1.22 тот же риск возникал при сохранении `&item`.

```go
// ✗ Bad (pre-1.22) — all pointers point to the same address
type Item struct{ Name string }
items := []Item{{Name: "a"}, {Name: "b"}}
var ptrs []*Item
for _, item := range items {
    ptrs = append(ptrs, &item) // all point to same loop variable
}
// ptrs[0].Name == "b", ptrs[1].Name == "b"

// ✓ Good — take address of the slice element directly
for i := range items {
    ptrs = append(ptrs, &items[i])
}
```

В новой семантике адрес переменной итерации корректен, но указывает на копию.
`&items[i]` указывает на элемент исходного массива и избегает этой копии;
выбор зависит от требуемого владения, а не только читаемости.

## Заголовок среза и общий массив

Срез содержит указатель, длину и ёмкость. Несколько срезов могут разделять массив.

```
a := make([]int, 3, 5)
┌─────┬─────┬─────┐
│ ptr │ len=3│cap=5│  ← slice header for a
└──┬──┴─────┴─────┘
   │
   ▼
┌───┬───┬───┬───┬───┐
│ 0 │ 0 │ 0 │   │   │  ← backing array (5 elements)
└───┴───┴───┴───┴───┘

b := a[1:2]
┌─────┬─────┬─────┐
│ ptr │ len=1│cap=4│  ← slice header for b (shares backing array)
└──┬──┴─────┴─────┘
   │ (points to a[1])
```

При запасе ёмкости `append` может изменить данные другого среза.
`a[:len(a):len(a)]` ограничивает ёмкость и заставляет непустой `append` выделить
новый массив; обычная запись в существующий элемент всё ещё меняет общий массив.

## Малый срез удерживает большой массив

Если надолго сохраняется небольшой фрагмент большого массива, копирование
через `slices.Clone` или `copy` позволяет освободить исходный массив.

```go
// ✗ Bad — small keeps the entire 1MB array alive
func getHeader(data []byte) []byte {
    return data[:64] // shares backing array with data
}

// ✓ Good — copy to release the large array
func getHeader(data []byte) []byte {
    header := make([]byte, 64)
    copy(header, data[:64])
    return header
}

// ✓ Good (Go 1.21+) — use slices.Clone
import "slices"

func getHeader(data []byte) []byte {
    return slices.Clone(data[:64])
}
```

Копируй ради известного времени жизни или защиты состояния, а не каждый возвращаемый срез.

## Clone из стандартной библиотеки: Go 1.21+

```go
import (
    "maps"
    "slices"
)

// Shallow copy a slice
clone := slices.Clone(original)

// Shallow copy a map
clone := maps.Clone(original)
```

`slices.Clone` и `maps.Clone` сохраняют nil и делают поверхностные копии.
Вложенные указатели и коллекции могут остаться общими.

## Порядок обхода map

Не полагайся на порядок итерации map. Для воспроизводимого вывода сортируй ключи.

```go
// ✗ Bad — output order changes between runs
m := map[string]int{"a": 1, "b": 2, "c": 3}
for k, v := range m {
    fmt.Printf("%s=%d ", k, v) // could be "b=2 a=1 c=3" or any permutation
}

// ✓ Good (Go 1.23+) — sort keys when order matters
keys := slices.Sorted(maps.Keys(m))
for _, k := range keys {
    fmt.Printf("%s=%d ", k, m[k])
}
```

## Удаление во время обхода

### Map

Удаление записи во время `range` разрешено языком.

```go
// ✓ Safe — defined behavior
for k, v := range m {
    if shouldDelete(v) {
        delete(m, k) // safe during range
    }
}
```

### Срез

При удалении из среза нужно учитывать изменение индексов; допустима фильтрация на месте.

```go
// ✗ Bad — skips elements after deletion
for i, v := range items {
    if shouldDelete(v) {
        items = append(items[:i], items[i+1:]...) // shifts elements, next iteration skips one
    }
}

// ✓ Good — iterate backwards
for i := len(items) - 1; i >= 0; i-- {
    if shouldDelete(items[i]) {
        items = append(items[:i], items[i+1:]...)
    }
}

// ✓ Good (Go 1.21+) — use slices.DeleteFunc
items = slices.DeleteFunc(items, shouldDelete)
```

Сокращение длины не очищает ссылочные элементы за новым концом среза.
Если из-за них удерживаются ресурсы, очисти хвост или выбери подходящий API.

## Сравнение

Срезы и map нельзя сравнивать друг с другом через `==`; сравнение с nil допустимо.
Для сопоставимых элементов используй `slices.Equal` и `maps.Equal`,
для собственной семантики — `EqualFunc`.

```go
import (
    "maps"
    "slices"
)

// ✓ Good (Go 1.21+)
slices.Equal(a, b)      // element-wise comparison
maps.Equal(m1, m2)      // key-value comparison

// For custom comparison
slices.EqualFunc(a, b, func(x, y Item) bool {
    return x.ID == y.ID
})
```

`slices.Equal` считает nil и пустой срез равными; учитывай это в контракте сериализации.
