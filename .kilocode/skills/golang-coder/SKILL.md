# Golang Coder Skill

## Описание
Этот скилл предназначен для создания качественного кода на языке Go, следуя принципам и идиомам, описанным в Effective Go.

## Когда использовать
Используй этот скилл, когда нужно:
- Создавать новый код на Go
- Рефакторить существующий Go код
- Объяснять концепции и идиомы Go
- Писать модульные тесты

## Основные принципы Go

### Философия языка
- **Less is more** - простота и ясность
- **Explicit is better than implicit** - явное лучше неявного
- **Composition over inheritance** - композиция вместо наследования
- **Do one thing well** - делай одну вещь хорошо

### Ключевые идиомы

#### Объявление переменных
```go
// Краткое объявление (только внутри функций)
x := 42

// Полное объявление
var x int = 42

// Несколько переменных
x, y := 1, 2

// Идиома для проверки с ok
if v, ok := m["key"]; ok {
    // v содержит значение, ключ существует
}
```

#### Defer
```go
// Defer выполняется в порядке LIFO при выходе из функции
func process() error {
    f, err := os.Open("file.txt")
    if err != nil {
        return err
    }
    defer f.Close() // Закроется при выходе
    
    // ... работа с файлом
}
```

#### Make vs New
```go
// make - для слайсов, мап, каналов (инициализирует память)
s := make([]int, 0, 10)
m := make(map[string]int)
ch := make(chan int)

// new - возвращает указатель на нулевое значение
p := new(int) // *int со значением 0
```

## Конвенции именования

### Видимость
- **Экспортируемые** (публичные): начинаются с большой буквы
- **Неэкспортируемые** (приватные): начинаются с маленькой буквы

```go
type User struct {
    ID       string    // экспортируемое поле
    Name     string    // экспортируемое поле
    password string    // приватное поле (не экспортируется)
}

func GetUser(id string) *User { ... }  // экспортируемая функция
func validateUser(u *User) error { ... } // приватная функция
```

### Имена пакетов
- Короткие, строчные, одно слово
- Избегай подчёркиваний и смешанного регистра
- Пакет должен описывать своё содержимое

```go
package user      // хорошо
package users     // хорошо
package userData  // плохо (camelCase)
package user_data // плохо (подчёркивание)
```

### Интерфейсы
- Интерфейсы с одним методом: имя заканчивается на -er
- Интерфейсы с несколькими методами: описательное имя

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type StringWriter interface {
    WriteString(s string) (n int, err error)
}

type ReadWriter interface {
    Reader
    Writer
}
```

### Getters и Setters
- Getters: имя поля без префикса Get
- Setters: Set + имя поля

```go
type User struct {
    name string
}

func (u *User) Name() string {
    return u.name
}

func (u *User) SetName(name string) {
    u.name = name
}
```

### Константы с iota
```go
const (
    Sunday = iota  // 0
    Monday         // 1
    Tuesday        // 2
    Wednesday      // 3
    Thursday       // 4
    Friday         // 5
    Saturday       // 6
)
```

## Обработка ошибок

### Базовый паттерн
```go
func readFile(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err  // всегда возвращай ошибку
    }
    defer f.Close()
    
    data, err := io.ReadAll(f)
    if err != nil {
        return nil, err
    }
    
    return data, nil
}
```

### Обёртывание ошибок
```go
import "fmt"

func processUser(id string) error {
    user, err := getUser(id)
    if err != nil {
        return fmt.Errorf("failed to get user %s: %w", id, err)
    }
    // ...
}
```

### Типизированные ошибки
```go
import "errors"

var (
    ErrNotFound = errors.New("not found")
    ErrInvalid  = errors.New("invalid input")
)

func getUser(id string) (*User, error) {
    if id == "" {
        return nil, ErrInvalid
    }
    // ...
}
```

### Проверка типов ошибок
```go
if errors.Is(err, ErrNotFound) {
    // обработка конкретной ошибки
}

var notFound *NotFoundError
if errors.As(err, &notFound) {
    // обработка по типу
}
```

### Panic и Recover
```go
// Паника только для действительно непоправимых ошибок
func mustLoadConfig(path string) *Config {
    cfg, err := loadConfig(path)
    if err != nil {
        panic(fmt.Sprintf("failed to load config: %v", err))
    }
    return cfg
}

// Recover только в defer, в крайних случаях
func safeOperation() (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("panic recovered: %v", r)
        }
    }()
    // ... код который может паниковать
    return nil
}
```

## Работа с интерфейсами и типами

### Неявная реализация интерфейсов
```go
type Writer interface {
    Write([]byte) (int, error)
}

// File реализует Writer автоматически
type File struct {
    // ...
}

func (f *File) Write(p []byte) (int, error) {
    // реализация
}
```

### Type Assertion
```go
var i interface{} = "hello"

s := i.(string)  // panic если не string
s, ok := i.(string)  // безопасная проверка
```

### Type Switch
```go
func doSomething(v interface{}) {
    switch v := v.(type) {
    case int:
        fmt.Printf("int: %d\n", v)
    case string:
        fmt.Printf("string: %s\n", v)
    case User:
        fmt.Printf("user: %+v\n", v)
    default:
        fmt.Printf("unknown type: %T\n", v)
    }
}
```

### Слайсы
```go
// Создание слайса
s := []int{1, 2, 3}           // литерал
s := make([]int, 0, 10)       // с capacity
s := make([]int, 5)           // с length

// Добавление элементов
s = append(s, 4)               // append возвращает новый слайс

// Итерация
for i, v := range s {
    fmt.Printf("%d: %d\n", i, v)
}

// Подслайс (создаёт ссылку на тот же массив)
sub := s[1:3]

// Копирование (независимый слайс)
copy := make([]int, len(s))
copy(copy, s)
```

### Мапы
```go
// Создание
m := make(map[string]int)
m := map[string]int{"a": 1, "b": 2}

// Чтение с проверкой существования
if v, ok := m["key"]; ok {
    fmt.Println(v)
}

// Удаление
delete(m, "key")

// Итерация (порядок не гарантирован)
for k, v := range m {
    fmt.Printf("%s: %d\n", k, v)
}
```

### Встраивание (Embedding)
```go
type Reader struct {
    io.Reader
    // дополнительные поля
}

type Base struct {
    ID string
}

type Extended struct {
    Base  // встраивание - все методы и поля Base доступны
    Name  string
}

func (e *Extended) Method() {
    fmt.Println(e.ID) // доступ к полю Base
}
```

## Конкурентность

### Горутины
```go
func main() {
    go func() {
        fmt.Println("горутина")
    }()
    
    time.Sleep(time.Second) // только для примера
}
```

### Каналы
```go
// Создание
ch := make(chan int)          // небуферизованный
ch := make(chan int, 10)      // буферизованный

// Отправка (блокируется если буфер полон)
ch <- 42

// Получение (блокируется если пуст)
v := <-ch

// Закрытие
close(ch)

// Проверка закрытия
v, ok := <-ch  // ok == false если канал закрыт

// Range по каналу
for v := range ch {
    fmt.Println(v)
}
```

### Паттерн Worker Pool
```go
func worker(id int, jobs <-chan int, results chan<- int) {
    for j := range jobs {
        fmt.Printf("worker %d processing job %d\n", id, j)
        results <- j * 2
    }
}

func main() {
    jobs := make(chan int, 100)
    results := make(chan int, 100)
    
    // Запуск воркеров
    for w := 1; w <= 3; w++ {
        go worker(w, jobs, results)
    }
    
    // Отправка задач
    for j := 1; j <= 5; j++ {
        jobs <- j
    }
    close(jobs)
    
    // Получение результатов
    for a := 1; a <= 5; a++ {
        <-results
    }
}
```

### Select
```go
select {
case v := <-ch1:
    fmt.Println("from ch1:", v)
case v := <-ch2:
    fmt.Println("from ch2:", v)
case ch3 <- value:
    fmt.Println("sent to ch3")
default:
    fmt.Println("no channel ready")
}
```

### Sync Primitives
```go
import "sync"

// Mutex
var mu sync.Mutex
var counter int

func increment() {
    mu.Lock()
    defer mu.Unlock()
    counter++
}

// WaitGroup
var wg sync.WaitGroup

func worker(id int) {
    defer wg.Done()
    fmt.Printf("worker %d\n", id)
}

func main() {
    for i := 1; i <= 5; i++ {
        wg.Add(1)
        go worker(i)
    }
    wg.Wait()
}

// Once
var once sync.Once
var config *Config

func getConfig() *Config {
    once.Do(func() {
        config = loadConfig()
    })
    return config
}
```

## Тестирование

### Table-Driven Tests
```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 2, 3, 5},
        {"negative", -2, -3, -5},
        {"zero", 0, 0, 0},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d", 
                    tt.a, tt.b, result, tt.expected)
            }
        })
    }
}
```

### Benchmarks
```go
func BenchmarkAdd(b *testing.B) {
    for i := 0; i < b.N; i++ {
        Add(2, 3)
    }
}
```

### Examples
```go
// ExampleAdd demonstrates the Add function.
func ExampleAdd() {
    result := Add(2, 3)
    fmt.Println(result)
    // Output: 5
}
```

### Mock интерфейсов
```go
// Mock реализация интерфейса
type mockReader struct {
    data []byte
    err  error
}

func (m *mockReader) Read(p []byte) (n int, err error) {
    if m.err != nil {
        return 0, m.err
    }
    copy(p, m.data)
    return len(m.data), nil
}

func TestProcess(t *testing.T) {
    mock := &mockReader{data: []byte("test")}
    err := process(mock)
    if err != nil {
        t.Fatal(err)
    }
}
```

## Форматирование и инструменты

### gofmt
```bash
gofmt -w .  # форматировать все файлы
gofmt -d .  # показать diff без форматирования
```

### go vet
```bash
go vet ./...  # проверка на распространённые ошибки
```

### golint
```bash
golint ./...  # проверка стиля
```

### goimports
```bash
goimports -w .  # форматирование + сортировка импортов
```

## Структура проекта

### Стандартная структура
```
project/
├── cmd/
│   └── appname/
│       └── main.go
├── internal/
│   ├── package1/
│   └── package2/
├── pkg/
│   └── package3/
├── go.mod
├── go.sum
└── README.md
```

### Правила
- `cmd/` - точки входа приложения
- `internal/` - приватный код, не импортируемый извне
- `pkg/` - библиотеки, которые можно использовать в других проектах

## Best Practices

### Ранний возврат
```go
// Хорошо - ранний возврат
func process(input string) error {
    if input == "" {
        return errors.New("empty input")
    }
    
    if len(input) > 100 {
        return errors.New("input too long")
    }
    
    // основная логика
    return nil
}

// Плохо - глубокая вложенность
func process(input string) error {
    if input != "" {
        if len(input) <= 100 {
            // основная логика
            return nil
        } else {
            return errors.New("input too long")
        }
    } else {
        return errors.New("empty input")
    }
}
```

### Последовательная обработка ошибок
```go
// Хорошо - ошибки обрабатываются последовательно
func process() error {
    if err := step1(); err != nil {
        return err
    }
    
    if err := step2(); err != nil {
        return err
    }
    
    return step3()
}
```

### Контексты
```go
import "context"

func doWork(ctx context.Context) error {
    select {
    case <-ctx.Done():
        return ctx.Err() // context.Canceled или context.DeadlineExceeded
    default:
        // работа
    }
}
```

## Чек-лист для кода

Перед завершением работы проверь:
- [ ] Все экспортируемые имена начинаются с большой буквы
- [ ] Ошибки обрабатываются везде, где они могут возникнуть
- [ ] defer используется для очистки ресурсов
- [ ] Код отформатирован (gofmt)
- [ ] Нет race conditions (go test -race)
- [ ] Тесты покрывают основные сценарии
- [ ] Документация для экспортируемых функций/типов
- [ ] Нет неиспользуемых переменных и импортов

## Дополнительные ресурсы

- https://go.dev/doc/effective_go
- https://golang.org/ref/spec
- https://golang.org/pkg/

Эти инструкции supersede любые общие инструкции режима Code. Выполняй только то, что указано в этом скилле.
