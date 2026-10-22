# Конвейеры и ограничение параллелизма

## Конвейер

Стадия получает значения из предыдущего канала, обрабатывает их и отправляет
в следующий. Стадия может состоять из одной или нескольких горутин.

```go
// Stage 1: Generate integers
func generate(ctx context.Context, nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for _, n := range nums {
            select {
            case out <- n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Stage 2: Square each integer
func square(ctx context.Context, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            select {
            case out <- n * n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Usage
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    ch := generate(ctx, 2, 3, 4)
    results := square(ctx, ch)

    for v := range results {
        fmt.Println(v) // 4, 9, 16
    }
}
```

Для отменяемого конвейера каждая блокирующая передача должна иметь путь отмены.
Каждая стадия закрывает собственный выход после завершения отправок.
Не создавай неограниченное число горутин; буфер выбирай по протоколу передачи.

## Fan-out и fan-in

Fan-out распределяет чтение одного канала между работниками.
Fan-in объединяет результаты нескольких каналов в общий выход.

```go
// Fan-out: N workers reading from the same input channel
func fanOut(ctx context.Context, in <-chan Task, workers int) <-chan Result {
    out := make(chan Result)
    var wg sync.WaitGroup

    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case task, ok := <-in:
                    if !ok {
                        return
                    }
                    select {
                    case out <- process(ctx, task):
                    case <-ctx.Done():
                        return
                    }
                case <-ctx.Done():
                    return
                }
            }
        }()
    }

    go func() {
        wg.Wait()
        close(out)
    }()
    return out
}
```

Объединение каналов должно завершаться после всех отправителей и поддерживать отмену.

```go
// Fan-in: merge multiple channels into one
func fanIn(ctx context.Context, channels ...<-chan Result) <-chan Result {
    out := make(chan Result)
    var wg sync.WaitGroup

    for _, ch := range channels {
        wg.Add(1)
        go func(c <-chan Result) {
            defer wg.Done()
            for v := range c {
                select {
                case out <- v:
                case <-ctx.Done():
                    return
                }
            }
        }(ch)
    }

    go func() {
        wg.Wait()
        close(out)
    }()
    return out
}
```

## Ограниченная группа errgroup

`errgroup.SetLimit` подходит для конечного набора независимых задач с общей
обработкой ошибки и лимитом конкурентности. Это не обязательная замена любого пула.

```go
func processAll(ctx context.Context, tasks []Task) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(10) // max 10 concurrent workers

    for _, task := range tasks {
        g.Go(func() error {
            return process(ctx, task)
        })
    }
    return g.Wait()
}
```

Отдельный пул нужен, когда важны состояние работника, соединения и буферы,
приоритеты, обратное давление или завершение уже взятых задач после закрытия входа.
`SetLimit` ограничивает число активных функций; ожидание внутри `Go` само по себе
не является отменяемым ожиданием свободного слота. Учитывай это в протоколе остановки.

## Семафор

Канал может ограничивать число одновременно выполняемых операций.

```go
func processAll(ctx context.Context, items []Item) error {
    sem := make(chan struct{}, 10) // semaphore of 10
    var wg sync.WaitGroup

    for _, item := range items {
        wg.Add(1)
        sem <- struct{}{} // acquire
        go func(item Item) {
            defer wg.Done()
            defer func() { <-sem }() // release
            process(ctx, item)
        }(item)
    }
    wg.Wait()
    return nil
}
```

Если ожидание лимита должно отменяться, получай разрешение через `select`
с `ctx.Done()`. При необходимости возврата ошибок рассмотри `errgroup`.

## Последовательные альтернативы

### Итераторы Go 1.23+

Для преобразований внутри процесса без необходимой конкурентности итератор
даёт ленивую обработку без горутин и каналов.

```go
func Filter[T any](seq iter.Seq[T], pred func(T) bool) iter.Seq[T] {
    return func(yield func(T) bool) {
        for v := range seq {
            if pred(v) {
                if !yield(v) {
                    return
                }
            }
        }
    }
}

func Map[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
    return func(yield func(U) bool) {
        for v := range seq {
            if !yield(f(v)) {
                return
            }
        }
    }
}
```

Итераторы подходят для последовательного источника или вычислений без пользы
от параллелизма. Канальные конвейеры уместны при перекрытии I/O, реальном
параллелизме или разных скоростях стадий.

### samber/ro

Пример показывает API дополнительной библиотеки. Используй его, если такая
зависимость уже выбрана и соответствует задаче; не устанавливай ради наличия конвейера.

```go
import "github.com/samber/ro"

emails, _ := ro.Collect( // ignore error
    ro.Pipe(
        ro.FromSlice(users),
        ro.Filter(func(u User) bool { return u.Active }),
        ro.Map(func(u User) string { return u.Email }),
    ),
)

```

Проверь актуальное API и модель выполнения библиотеки перед применением.

## Обнаружение утечек

`goleak` может дополнять проверки пакета с горутинами, но не является обязательной
зависимостью каждого проекта. Политику тестирования определяют правила проекта.

```go
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

## Типичные ошибки

| Ошибка | Исправление |
| --- | --- |
| Стадия не может завершить блокирующее ожидание | Предусмотреть отмену или гарантированное событие завершения |
| Выход не закрывается после всех отправок | Закрыть у владельца отправки |
| Неограниченный запуск | Ограничить работу группой, пулом или семафором |
| Общие изменяемые данные в канале | Передать владение, синхронизировать или скопировать по необходимости |
| Отправка блокируется после ухода получателя | Согласовать остановку и отмену отправки |
