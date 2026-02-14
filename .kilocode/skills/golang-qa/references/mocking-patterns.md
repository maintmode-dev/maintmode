# Mocking Patterns

Comprehensive guide to mocking dependencies in Go tests.

## Interface Design for Testability

```go
// Good - interface for dependency
type UserRepository interface {
    GetUser(id int) (*User, error)
    SaveUser(user *User) error
}

type UserService struct {
    repo UserRepository // Depend on interface
}

func NewUserService(repo UserRepository) *UserService {
    return &UserService{repo: repo}
}
```

## Mock with testify/mock

```go
import "github.com/stretchr/testify/mock"

type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) GetUser(id int) (*User, error) {
    args := m.Called(id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserRepository) SaveUser(user *User) error {
    args := m.Called(user)
    return args.Error(0)
}

func TestUserService(t *testing.T) {
    // Setup mock
    mockRepo := new(MockUserRepository)
    mockRepo.On("GetUser", 123).Return(&User{ID: 123, Name: "John"}, nil)

    // Test
    service := NewUserService(mockRepo)
    user, err := service.GetUserByID(123)

    // Assertions
    assert.NoError(t, err)
    assert.Equal(t, "John", user.Name)
    mockRepo.AssertExpectations(t)
}
```

## Manual Mock Implementation

```go
type mockUserRepository struct {
    getUserFunc func(id int) (*User, error)
    saveUserFunc func(user *User) error
}

func (m *mockUserRepository) GetUser(id int) (*User, error) {
    if m.getUserFunc != nil {
        return m.getUserFunc(id)
    }
    return nil, nil
}

func (m *mockUserRepository) SaveUser(user *User) error {
    if m.saveUserFunc != nil {
        return m.saveUserFunc(user)
    }
    return nil
}

func TestUserServiceManual(t *testing.T) {
    mock := &mockUserRepository{
        getUserFunc: func(id int) (*User, error) {
            if id == 123 {
                return &User{ID: 123, Name: "John"}, nil
            }
            return nil, errors.New("not found")
        },
    }

    service := NewUserService(mock)
    user, err := service.GetUserByID(123)

    assert.NoError(t, err)
    assert.Equal(t, "John", user.Name)
}
```

## Mocking HTTP Clients

```go
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}

type MockHTTPClient struct {
    mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
    args := m.Called(req)
    return args.Get(0).(*http.Response), args.Error(1)
}

func TestAPIClient(t *testing.T) {
    mockClient := new(MockHTTPClient)

    // Mock response
    response := &http.Response{
        StatusCode: 200,
        Body:       io.NopCloser(strings.NewReader(`{"id":123,"name":"John"}`)),
    }
    mockClient.On("Do", mock.Anything).Return(response, nil)

    client := NewAPIClient(mockClient)
    user, err := client.GetUser(123)

    assert.NoError(t, err)
    assert.Equal(t, "John", user.Name)
    mockClient.AssertExpectations(t)
}
```

## Mocking with httptest

For testing HTTP handlers without mocking:

```go
func TestUserHandler(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(User{ID: 123, Name: "John"})
    }))
    defer server.Close()

    // Test client using test server
    client := NewAPIClient(server.URL)
    user, err := client.GetUser(123)

    assert.NoError(t, err)
    assert.Equal(t, "John", user.Name)
}
```

## Mocking Time

```go
type Clock interface {
    Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
    return time.Now()
}

type mockClock struct {
    currentTime time.Time
}

func (m mockClock) Now() time.Time {
    return m.currentTime
}

func TestTimeDependent(t *testing.T) {
    fixedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
    clock := mockClock{currentTime: fixedTime}

    service := NewService(clock)
    result := service.DoSomethingWithTime()

    assert.Equal(t, fixedTime, result.Timestamp)
}
```

## Dependency Injection Patterns

### Constructor Injection

```go
type Service struct {
    repo   UserRepository
    cache  Cache
    logger Logger
}

func NewService(repo UserRepository, cache Cache, logger Logger) *Service {
    return &Service{
        repo:   repo,
        cache:  cache,
        logger: logger,
    }
}
```

### Functional Options

```go
type Service struct {
    repo   UserRepository
    cache  Cache
    logger Logger
}

type Option func(*Service)

func WithCache(cache Cache) Option {
    return func(s *Service) {
        s.cache = cache
    }
}

func WithLogger(logger Logger) Option {
    return func(s *Service) {
        s.logger = logger
    }
}

func NewService(repo UserRepository, opts ...Option) *Service {
    s := &Service{repo: repo}
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// In tests
func TestService(t *testing.T) {
    mockRepo := new(MockUserRepository)
    mockCache := new(MockCache)

    service := NewService(mockRepo, WithCache(mockCache))
    // Test service
}
```

## Best Practices

1. **Design for testability** - Use interfaces for dependencies
2. **Inject dependencies** - Don't create them inside functions
3. **Keep mocks simple** - Don't over-complicate mock logic
4. **Use testify/mock for complex mocks** - Built-in assertion support
5. **Verify mock expectations** - Always call AssertExpectations()
6. **Mock external dependencies only** - Don't mock internal logic
7. **Use real implementations when possible** - Prefer integration tests
8. **Keep test setup minimal** - Only mock what's necessary

## Common Pitfalls

### ❌ Don't mock everything

```go
// Bad - mocking internal types
mockUser := new(MockUser)
mockOrder := new(MockOrder)
```

### ✅ Mock external dependencies

```go
// Good - mocking external services
mockRepo := new(MockRepository)
mockAPI := new(MockAPIClient)
```

### ❌ Don't test the mock

```go
// Bad - testing mock behavior
mock.On("GetUser", 123).Return(user, nil)
result, _ := mock.GetUser(123)
assert.Equal(t, user, result) // Testing the mock!
```

### ✅ Test real behavior

```go
// Good - testing service with mock
mockRepo.On("GetUser", 123).Return(user, nil)
service := NewService(mockRepo)
result, _ := service.ProcessUser(123) // Testing real logic
assert.Equal(t, expected, result)
```
