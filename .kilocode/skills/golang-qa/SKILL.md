# Golang Automation QA Skill

## Описание
Этот скилл предназначен для создания автоматизированных тестов на языке Go, включая unit, integration и end-to-end тестирование, а также инструменты для QA автоматизации.

## Когда использовать
Используй этот скилл, когда нужно:
- Писать unit тесты
- Создавать integration тесты
- Написать E2E тесты для API
- Настраивать test fixtures и mocks
- Работать с test контейнерами
- Настраивать CI/CD для тестов
- Писать BDD тесты
- Создавать нагрузочное тестирование

## Основы тестирования в Go

### t.Parallel() - Параллельное выполнение тестов

> **Важно:** Всегда добавляй `t.Parallel()` в начало каждой тестовой функции для параллельного выполнения тестов. Это значительно ускоряет выполнение тестов.

```go
func TestMyFunction(t *testing.T) {
    t.Parallel()  // Всегда добавляй эту строку первой
    
    // Тестовый код
    result := MyFunction()
    require.Equal(t, expected, result)
}
```

#### Table-Driven Tests с t.Parallel()

При использовании table-driven tests с `t.Parallel()` необходимо захватывать переменную цикла:

```go
func TestAdd(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 2, 3, 5},
        {"negative", -2, -3, -5},
    }
    
    for _, tt := range tests {
        tt := tt // Важно: захват переменной для параллельного выполнения
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()  // Добавляем в каждый subtest
            result := Add(tt.a, tt.b)
            require.Equal(t, tt.expected, result)
        })
    }
}
```

#### Почему t.Parallel() важен?

1. **Скорость:** Тесты выполняются параллельно, что значительно ускоряет их выполнение
2. **Изоляция:** Параллельное выполнение помогает выявить race conditions и проблемы с изоляцией тестов
3. **Лучшие практики:** Это стандарт Go для написания эффективных тестов

#### Исключения

Не используй `t.Parallel()` в следующих случаях:
- Тесты, которые должны выполняться последовательно (например, проверка состояния глобального ресурса)
- Benchmark тесты (они используют свой механизм параллелизации)
- Helper функции (они не являются тестами)

```go
// Helper функция - не используем t.Parallel()
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()  // Используем t.Helper() вместо t.Parallel()
    // ... код настройки
}
```

### Базовый тест
```go
package calculator

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestAdd(t *testing.T) {
    t.Parallel()
    
    result := Add(2, 3)
    expected := 5
    
    require.Equal(t, expected, result, "Add(2, 3) should equal 5")
}
```

### Table-Driven Tests
```go
func TestAdd(t *testing.T) {
    t.Parallel()
    
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 2, 3, 5},
        {"negative", -2, -3, -5},
        {"mixed", -2, 3, 1},
        {"zero", 0, 0, 0},
    }
    
    for _, tt := range tests {
        tt := tt // захват переменной для параллельного выполнения
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            result := Add(tt.a, tt.b)
            require.Equal(t, tt.expected, result, "Add(%d, %d) should equal %d",
                tt.a, tt.b, tt.expected)
        })
    }
}
```

### Subtests
```go
func TestProcessUser(t *testing.T) {
    t.Parallel()
    
    t.Run("valid user", func(t *testing.T) {
        t.Parallel()
        user := User{Name: "John", Email: "john@example.com"}
        err := ProcessUser(user)
        require.NoError(t, err, "should not return error for valid user")
    })
    
    t.Run("invalid email", func(t *testing.T) {
        t.Parallel()
        user := User{Name: "John", Email: "invalid"}
        err := ProcessUser(user)
        require.Error(t, err, "should return error for invalid email")
    })
}
```

### Setup и Teardown
```go
func TestMain(m *testing.M) {
    // Setup
    setup()
    
    // Запуск тестов
    code := m.Run()
    
    // Teardown
    teardown()
    
    os.Exit(code)
}

func setup() {
    // Инициализация тестовой БД
    // Загрузка тестовых данных
}

func teardown() {
    // Очистка тестовой БД
    // Закрытие соединений
}
```

### Test Helpers
```go
// testutils/helpers.go
package testutils

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func RequireEqual[T comparable](t *testing.T, got, want T) {
    t.Helper()
    require.Equal(t, want, got)
}

func RequireNoError(t *testing.T, err error) {
    t.Helper()
    require.NoError(t, err)
}

func RequireError(t *testing.T, err error, msg string) {
    t.Helper()
    require.Error(t, err)
    require.Equal(t, msg, err.Error())
}
```

## Testify Assertions

> **Важно:** По умолчанию используй **require** для всех проверок. **assert** используется только в исключительных случаях (например, когда нужно проверить несколько условий в одном тесте и продолжить выполнение после неудачи одной из проверок).

### Основные Assertions

| Assertion | Описание |
|-----------|----------|
| `require.Equal(t, expected, actual)` | Проверяет равенство значений |
| `require.NotEqual(t, notExpected, actual)` | Проверяет неравенство значений |
| `require.Nil(t, object)` | Проверяет, что объект равен nil |
| `require.NotNil(t, object)` | Проверяет, что объект не равен nil |
| `require.True(t, value)` | Проверяет, что значение равно true |
| `require.False(t, value)` | Проверяет, что значение равно false |
| `require.Empty(t, object)` | Проверяет, что объект пустой |
| `require.NotEmpty(t, object)` | Проверяет, что объект не пустой |
| `require.Zero(t, value)` | Проверяет, что значение равно нулю |
| `require.NotZero(t, value)` | Проверяет, что значение не равно нулю |

### Assertions для ошибок

| Assertion | Описание |
|-----------|----------|
| `require.NoError(t, err)` | Проверяет, что ошибки нет |
| `require.Error(t, err)` | Проверяет, что ошибка есть |
| `require.ErrorIs(t, err, target)` | Проверяет, что ошибка соответствует target |
| `require.ErrorAs(t, err, target)` | Проверяет, что ошибка приводится к типу |
| `require.ErrorContains(t, err, substring)` | Проверяет, что ошибка содержит подстроку |

### Assertions для строк

| Assertion | Описание |
|-----------|----------|
| `require.Contains(t, string, substring)` | Проверяет, что строка содержит подстроку |
| `require.NotContains(t, string, substring)` | Проверяет, что строка не содержит подстроку |
| `require.HasPrefix(t, string, prefix)` | Проверяет префикс |
| `require.HasSuffix(t, string, suffix)` | Проверяет суффикс |
| `require.Regexp(t, regex, string)` | Проверяет совпадение с регулярным выражением |

### Assertions для коллекций

| Assertion | Описание |
|-----------|----------|
| `require.Len(t, object, length)` | Проверяет длину объекта |
| `require.ElementsMatch(t, list1, list2)` | Проверяет, что списки содержат одинаковые элементы |
| `require.Subset(t, subset, set)` | Проверяет, что subset является подмножеством set |
| `require.NotSubset(t, subset, set)` | Проверяет, что subset не является подмножеством set |
| `require.Contains(t, list, element)` | Проверяет, что список содержит элемент |
| `require.NotContains(t, list, element)` | Проверяет, что список не содержит элемент |

### Assertions для чисел

| Assertion | Описание |
|-----------|----------|
| `require.InDelta(t, expected, actual, delta)` | Проверяет, что значения отличаются не более чем на delta |
| `require.InEpsilon(t, expected, actual, epsilon)` | Проверяет относительную разницу |
| `require.Greater(t, a, b)` | Проверяет, что a > b |
| `require.GreaterOrEqual(t, a, b)` | Проверяет, что a >= b |
| `require.Less(t, a, b)` | Проверяет, что a < b |
| `require.LessOrEqual(t, a, b)` | Проверяет, что a <= b |
| `require.Positive(t, x)` | Проверяет, что x > 0 |
| `require.Negative(t, x)` | Проверяет, что x < 0 |

### Assertions для типов

| Assertion | Описание |
|-----------|----------|
| `require.IsType(t, expectedType, object)` | Проверяет тип объекта |
| `require.Implements(t, interfaceObject, object)` | Проверяет, что объект реализует интерфейс |
| `require.Panics(t, func())` | Проверяет, что функция паникует |
| `require.NotPanics(t, func())` | Проверяет, что функция не паникует |
| `require.PanicsWithValue(t, expected, func())` | Проверяет панику с конкретным значением |
| `require.PanicsWithError(t, errString, func())` | Проверяет панику с конкретным сообщением |

### Assertions для JSON

| Assertion | Описание |
|-----------|----------|
| `require.JSONEq(t, expected, actual)` | Сравнивает JSON строки |
| `require.YAMLEq(t, expected, actual)` | Сравнивает YAML строки |

### Assertions для файлов

| Assertion | Описание |
|-----------|----------|
| `require.FileExists(t, path)` | Проверяет, что файл существует |
| `require.NoFileExists(t, path)` | Проверяет, что файл не существует |
| `require.DirExists(t, path)` | Проверяет, что директория существует |
| `require.NoDirExists(t, path)` | Проверяет, что директория не существует |

### Assertions для HTTP

| Assertion | Описание |
|-----------|----------|
| `httpassert.Equal(t, expected, actual)` | Сравнивает HTTP объекты |
| `httpassert.EqualHeaders(t, expected, actual)` | Сравнивает заголовки |

### Различия между assert и require

> **Правило:** По умолчанию используй **require** для всех проверок.

- **require** (по умолчанию): Тест останавливается немедленно после неудачи проверки. Используй для всех проверок, особенно для критических операций (setup, setup errors, ошибки при получении данных).
- **assert** (исключительный случай): Тест продолжается после неудачи проверки. Используй только в исключительных случаях, когда нужно проверить несколько условий в одном тесте и увидеть все неудачные проверки (например, при проверке нескольких полей в цикле).

```go
// Стандартный подход - используем require для всех проверок
func TestUserCreation(t *testing.T) {
    user, err := CreateUser("John", "john@example.com")
    require.NoError(t, err, "failed to create user")
    require.NotNil(t, user, "user should not be nil")
    require.Equal(t, "John", user.Name, "user name should match")
    require.Equal(t, "john@example.com", user.Email, "user email should match")
}

// Исключительный случай - используем assert для проверки нескольких условий в цикле
func TestValidateMultipleFields(t *testing.T) {
    fields := []Field{
        {Name: "email", Value: "invalid-email"},
        {Name: "age", Value: "invalid"},
        {Name: "name", Value: ""},
    }
    
    for _, field := range fields {
        err := ValidateField(field)
        // Используем assert, чтобы увидеть все ошибки валидации
        assert.Error(t, err, "field %s should be invalid", field.Name)
    }
}
```

## Mocking

### Интерфейсы для тестирования
```go
// production code
type UserRepository interface {
    GetByID(id string) (*User, error)
    Create(user *User) error
    Update(user *User) error
    Delete(id string) error
}

type UserService struct {
    repo UserRepository
}

func (s *UserService) GetUser(id string) (*User, error) {
    return s.repo.GetByID(id)
}
```

### Mock реализация вручную
```go
// test/mocks/user_repo_mock.go
package mocks

type MockUserRepository struct {
    users map[string]*User
    err   error
}

func NewMockUserRepository() *MockUserRepository {
    return &MockUserRepository{
        users: make(map[string]*User),
    }
}

func (m *MockUserRepository) GetByID(id string) (*User, error) {
    if m.err != nil {
        return nil, m.err
    }
    return m.users[id], nil
}

func (m *MockUserRepository) Create(user *User) error {
    if m.err != nil {
        return m.err
    }
    m.users[user.ID] = user
    return nil
}

func (m *MockUserRepository) SetError(err error) {
    m.err = err
}

func (m *MockUserRepository) AddUser(user *User) {
    m.users[user.ID] = user
}
```

### Использование mock в тесте
```go
import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestUserService_GetUser(t *testing.T) {
    t.Parallel()
    
    mockRepo := mocks.NewMockUserRepository()
    mockRepo.AddUser(&User{ID: "123", Name: "John"})
    
    service := &UserService{repo: mockRepo}
    
    user, err := service.GetUser("123")
    require.NoError(t, err)
    require.Equal(t, "John", user.Name)
}

func TestUserService_GetUser_NotFound(t *testing.T) {
    t.Parallel()
    
    mockRepo := mocks.NewMockUserRepository()
    mockRepo.SetError(ErrNotFound)
    
    service := &UserService{repo: mockRepo}
    
    _, err := service.GetUser("999")
    require.Error(t, err)
    require.ErrorIs(t, err, ErrNotFound)
}
```

### gomock
```go
//go:generate mockgen -source=user_service.go -destination=mocks/user_service_mock.go

import (
    "testing"
    "github.com/golang/mock/gomock"
    "github.com/stretchr/testify/require"
)

func TestUserService_GetUser(t *testing.T) {
    t.Parallel()
    
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    mockRepo := mocks.NewMockUserRepository(ctrl)
    
    mockRepo.EXPECT().
        GetByID("123").
        Return(&User{ID: "123", Name: "John"}, nil)
    
    service := &UserService{repo: mockRepo}
    
    user, err := service.GetUser("123")
    require.NoError(t, err)
    require.Equal(t, "John", user.Name)
}
```

### testify/mock
```go
import (
    "testing"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"
)

type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) GetByID(id string) (*User, error) {
    args := m.Called(id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*User), args.Error(1)
}

func TestUserService_GetUser(t *testing.T) {
    t.Parallel()
    
    mockRepo := new(MockUserRepository)
    
    mockRepo.On("GetByID", "123").Return(&User{ID: "123", Name: "John"}, nil)
    
    service := &UserService{repo: mockRepo}
    
    user, err := service.GetUser("123")
    require.NoError(t, err)
    require.Equal(t, "John", user.Name)
    
    mockRepo.AssertExpectations(t)
}
```

## HTTP Testing

### httptest для API тестов
```go
import (
    "net/http"
    "net/http/httptest"
    "encoding/json"
    "strings"
    "github.com/stretchr/testify/require"
)

func TestCreateUserHandler(t *testing.T) {
    t.Parallel()
    
    handler := CreateUserHandler()
    
    body := `{"name":"John","email":"john@example.com"}`
    req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    
    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)
    
    require.Equal(t, http.StatusCreated, w.Code, "should return 201 status")
    
    var user User
    err := json.NewDecoder(w.Body).Decode(&user)
    require.NoError(t, err)
    
    require.Equal(t, "John", user.Name, "user name should be John")
}
```

### Test HTTP Server
```go
func setupTestServer() *httptest.Server {
    mux := http.NewServeMux()
    mux.HandleFunc("/users", getUsersHandler)
    mux.HandleFunc("/users/", getUserHandler)
    
    return httptest.NewServer(mux)
}

func TestAPI(t *testing.T) {
    t.Parallel()
    
    server := setupTestServer()
    defer server.Close()
    
    // Тест GET /users
    resp, err := http.Get(server.URL + "/users")
    require.NoError(t, err)
    defer resp.Body.Close()
    
    require.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 status")
    
    var users []User
    err = json.NewDecoder(resp.Body).Decode(&users)
    require.NoError(t, err)
    
    require.NotEmpty(t, users, "users list should not be empty")
}
```

### Test HTTP Middleware
```go
func TestAuthMiddleware(t *testing.T) {
    t.Parallel()
    
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })
    
    authMiddleware := AuthMiddleware()
    protectedHandler := authMiddleware(handler)
    
    tests := []struct {
        name       string
        token      string
        wantStatus int
    }{
        {"valid token", "valid-token", http.StatusOK},
        {"no token", "", http.StatusUnauthorized},
        {"invalid token", "invalid", http.StatusUnauthorized},
    }
    
    for _, tt := range tests {
        tt := tt // захват переменной для параллельного выполнения
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            req := httptest.NewRequest(http.MethodGet, "/", nil)
            if tt.token != "" {
                req.Header.Set("Authorization", tt.token)
            }
            
            w := httptest.NewRecorder()
            protectedHandler.ServeHTTP(w, req)
            
            require.Equal(t, tt.wantStatus, w.Code,
                "should return %d status for %s", tt.wantStatus, tt.name)
        })
    }
}
```

## Database Testing

### Test Database Setup
```go
// test/db/testdb.go
package testdb

import (
    "testing"
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
    "github.com/stretchr/testify/require"
)

func SetupTestDB(t *testing.T) *pgxpool.Pool {
    ctx := context.Background()
    
    // Запуск PostgreSQL контейнера
    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image:        "postgres:15-alpine",
            ExposedPorts: []string{"5432/tcp"},
            Env: map[string]string{
                "POSTGRES_USER":     "test",
                "POSTGRES_PASSWORD": "test",
                "POSTGRES_DB":       "testdb",
            },
            WaitingFor: wait.ForLog("database system is ready to accept connections"),
        },
        Started: true,
    })
    require.NoError(t, err, "failed to start postgres container")
    
    t.Cleanup(func() {
        container.Terminate(ctx)
    })
    
    // Получение порта
    host, err := container.Host(ctx)
    require.NoError(t, err, "failed to get container host")
    
    port, err := container.MappedPort(ctx, "5432")
    require.NoError(t, err, "failed to get container port")
    
    // Подключение к БД
    connString := fmt.Sprintf("postgres://test:test@%s:%s/testdb", host, port.Port())
    pool, err := pgxpool.New(ctx, connString)
    require.NoError(t, err, "failed to connect to database")
    
    // Запуск миграций
    runMigrations(ctx, pool)
    
    return pool
}
```

### Test Fixtures
```go
// test/fixtures/users.go
package fixtures

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/stretchr/testify/require"
)

func CreateUser(ctx context.Context, pool *pgxpool.Pool, user User) error {
    const query = `
        INSERT INTO users (id, name, email) 
        VALUES ($1, $2, $3)
    `
    _, err := pool.Exec(ctx, query, user.ID, user.Name, user.Email)
    return err
}

func CreateUsers(ctx context.Context, pool *pgxpool.Pool, users []User) error {
    const query = `
        INSERT INTO users (id, name, email) 
        VALUES ($1, $2, $3)
    `
    
    for _, user := range users {
        if err := CreateUser(ctx, pool, user); err != nil {
            return err
        }
    }
    return nil
}

func ClearUsers(ctx context.Context, pool *pgxpool.Pool) error {
    const query = `DELETE FROM users`
    _, err := pool.Exec(ctx, query)
    return err
}

// Helper для тестов с автоматической очисткой
func CreateUserWithCleanup(t *testing.T, ctx context.Context, pool *pgxpool.Pool, user User) {
    t.Helper()
    err := CreateUser(ctx, pool, user)
    require.NoError(t, err, "failed to create test user")
    
    t.Cleanup(func() {
        const deleteQuery = `DELETE FROM users WHERE id = $1`
        pool.Exec(ctx, deleteQuery, user.ID)
    })
}
```

### Integration Tests с БД
```go
import (
    "context"
    "testing"
    "github.com/google/uuid"
    "github.com/stretchr/testify/require"
    "testproject/test/db"
    "testproject/test/fixtures"
)

func TestUserRepository_Integration(t *testing.T) {
    t.Parallel()
    
    ctx := context.Background()
    pool := testdb.SetupTestDB(t)
    
    repo := NewUserRepository(pool)
    
    t.Run("create and get user", func(t *testing.T) {
        t.Parallel()
        user := User{
            ID:    uuid.New().String(),
            Name:  "John",
            Email: "john@example.com",
        }
        
        // Create
        err := repo.Create(ctx, &user)
        require.NoError(t, err, "failed to create user")
        
        // Get
        got, err := repo.GetByID(ctx, user.ID)
        require.NoError(t, err, "failed to get user")
        require.Equal(t, user.Name, got.Name, "user name should match")
        require.Equal(t, user.Email, got.Email, "user email should match")
    })
    
    t.Run("get not found", func(t *testing.T) {
        t.Parallel()
        _, err := repo.GetByID(ctx, "non-existent")
        require.Error(t, err, "should return error for non-existent user")
    })
}
```

## Testcontainers

### PostgreSQL контейнер
```go
import (
    "context"
    "testing"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupPostgres(t *testing.T) (*pgxpool.Pool, func()) {
    ctx := context.Background()
    
    container, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:15-alpine"),
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
    )
    require.NoError(t, err, "failed to start postgres container")
    
    connStr, err := container.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err, "failed to get connection string")
    
    pool, err := pgxpool.New(ctx, connStr)
    require.NoError(t, err, "failed to create connection pool")
    
    cleanup := func() {
        pool.Close()
        container.Terminate(ctx)
    }
    
    return pool, cleanup
}
```

### Redis контейнер
```go
import (
    "context"
    "testing"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go/modules/redis"
)

func setupRedis(t *testing.T) (*redis.Client, func()) {
    ctx := context.Background()
    
    container, err := redis.RunContainer(ctx)
    require.NoError(t, err, "failed to start redis container")
    
    endpoint, err := container.Endpoint(ctx, "")
    require.NoError(t, err, "failed to get redis endpoint")
    
    client := redis.NewClient(&redis.Options{
        Addr: endpoint,
    })
    
    cleanup := func() {
        client.Close()
        container.Terminate(ctx)
    }
    
    return client, cleanup
}
```

### Комбинированная setup
```go
import (
    "context"
    "testing"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/modules/redis"
)

func setupTestEnvironment(t *testing.T) (*pgxpool.Pool, *redis.Client) {
    ctx := context.Background()
    
    // PostgreSQL
    pgContainer, err := postgres.RunContainer(ctx,
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
    )
    require.NoError(t, err)
    t.Cleanup(func() { pgContainer.Terminate(ctx) })
    
    pgConnStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)
    
    pgPool, err := pgxpool.New(ctx, pgConnStr)
    require.NoError(t, err)
    
    // Redis
    redisContainer, err := redis.RunContainer(ctx)
    require.NoError(t, err)
    t.Cleanup(func() { redisContainer.Terminate(ctx) })
    
    redisEndpoint, err := redisContainer.Endpoint(ctx, "")
    require.NoError(t, err)
    
    redisClient := redis.NewClient(&redis.Options{Addr: redisEndpoint})
    
    return pgPool, redisClient
}
```

## BDD Testing

### Ginkgo
```go
package user_test

import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("UserService", func() {
    var (
        service *UserService
        repo    *MockUserRepository
    )
    
    BeforeEach(func() {
        repo = NewMockUserRepository()
        service = NewUserService(repo)
    })
    
    Describe("GetUser", func() {
        Context("when user exists", func() {
            It("returns the user", func() {
                expectedUser := &User{ID: "123", Name: "John"}
                repo.AddUser(expectedUser)
                
                user, err := service.GetUser("123")
                
                Expect(err).NotTo(HaveOccurred())
                Expect(user).To(Equal(expectedUser))
            })
        })
        
        Context("when user does not exist", func() {
            It("returns an error", func() {
                _, err := service.GetUser("999")
                
                Expect(err).To(HaveOccurred())
                Expect(err).To(MatchError(ErrNotFound))
            })
        })
    })
})
```

### Gomega assertions
```go
import "github.com/onsi/gomega"

Expect(actual).To(Equal(expected))
Expect(actual).To(BeNil())
Expect(actual).NotTo(BeNil())
Expect(actual).To(HaveOccurred())
Expect(actual).NotTo(HaveOccurred())
Expect(actual).To(HaveLen(3))
Expect(actual).To(ContainElement("value"))
Expect(actual).To(BeNumerically(">", 10))
Expect(actual).To(MatchRegexp(`^\w+$`))
```

## Load Testing

### Vegeta
```go
package main

import (
    "fmt"
    "github.com/tsenart/vegeta/v12/lib"
    "time"
)

func main() {
    rate := vegeta.Rate{Freq: 100, Per: time.Second}
    duration := 30 * time.Second
    targeter := vegeta.NewStaticTargeter(vegeta.Target{
        Method: "GET",
        URL:    "http://localhost:8080/users",
    })
    
    attacker := vegeta.NewAttacker()
    
    var metrics vegeta.Metrics
    for res := range attacker.Attack(targeter, rate, duration, "Big Bang!") {
        metrics.Add(res)
    }
    metrics.Close()
    
    fmt.Printf("Requests: %d\n", metrics.Requests)
    fmt.Printf("Success: %.2f%%\n", metrics.Success*100)
    fmt.Printf("Latencies:\n")
    fmt.Printf("  50th: %s\n", metrics.Latencies.P50)
    fmt.Printf("  95th: %s\n", metrics.Latencies.P95)
    fmt.Printf("  99th: %s\n", metrics.Latencies.P99)
}
```

### Benchmark тесты
```go
func BenchmarkGetUser(b *testing.B) {
    repo := setupBenchmarkRepo()
    service := NewUserService(repo)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        service.GetUser("123")
    }
}

func BenchmarkParallelGetUser(b *testing.B) {
    repo := setupBenchmarkRepo()
    service := NewUserService(repo)
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            service.GetUser("123")
        }
    })
}
```

## Test Reporting

### Coverprofile
```bash
# Запуск тестов с покрытием
go test -coverprofile=coverage.out ./...

# Просмотр покрытия
go tool cover -func=coverage.out

# HTML отчёт
go tool cover -html=coverage.out -o coverage.html
```

### Test JSON output
```bash
go test -json ./... > test-results.json
```

### JUnit XML
```bash
go install github.com/jstemmer/go-junit-report/v2@latest
go test -v ./... 2>&1 | go-junit-report -set-exit-code > report.xml
```

## CI/CD Integration

### GitHub Actions
```yaml
name: Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Download dependencies
      run: go mod download
    
    - name: Run tests
      run: go test -v -race -coverprofile=coverage.out ./...
    
    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        files: ./coverage.out
    
    - name: Run linter
      uses: golangci/golangci-lint-action@v3
      with:
        version: latest
```

## Чек-лист для QA автоматизации

Перед завершением работы проверь:
- [ ] Тесты изолированы и не зависят друг от друга
- [ ] **t.Parallel() добавлен во все тестовые функции**
- [ ] **tt := tt добавлен для захвата переменной в table-driven tests с t.Parallel()**
- [ ] Используются table-driven tests для множества сценариев
- [ ] Mock интерфейсов вместо реальных зависимостей
- [ ] Testcontainers для integration тестов
- [ ] Фикстуры для подготовки тестовых данных
- [ ] Cleanup в defer или t.Cleanup
- [ ] Race detection включён (-race)
- [ ] Покрытие кода тестами (цель >80%)
- [ ] Параметризованные тесты для граничных значений
- [ ] Error сценарии протестированы
- [ ] CI/CD настроен для автоматического запуска тестов
- [ ] Логирование тестов для отладки
- [ ] Benchmarks для критического кода
- [ ] **Используется testify/require для всех проверок по умолчанию**
- [ ] **assert используется только в исключительных случаях**
- [ ] Используется testify/require для всех проверок по умолчанию
- [ ] assert используется только в исключительных случаях (например, для проверки нескольких условий в цикле)

## Дополнительные ресурсы

- https://go.dev/doc/tutorial/add-a-test
- https://github.com/golang/go/wiki/TableDrivenTests
- https://github.com/testcontainers/testcontainers-go
- https://onsi.github.io/ginkgo/
- https://onsi.github.io/gomega/
- https://github.com/stretchr/testify
- https://pkg.go.dev/github.com/stretchr/testify/assert
- https://pkg.go.dev/github.com/stretchr/testify/require

Эти инструкции supersede любые общие инструкции режима Code. Выполняй только то, что указано в этом скилле.
