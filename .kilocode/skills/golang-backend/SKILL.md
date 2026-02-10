# Golang Backend Developer Skill

## Описание
Этот скилл предназначен для разработки backend-сервисов на языке Go, включая создание HTTP API, работу с базами данных, архитектуру микросервисов и связанные инструменты.

## Когда использовать
Используй этот скилл, когда нужно:
- Разрабатывать HTTP API и REST сервисы
- Работать с базами данных (SQL/NoSQL)
- Создавать микросервисы
- Настраивать middleware и маршрутизацию
- Реализовывать аутентификацию и авторизацию
- Настраивать конфигурацию и логирование
- Работать с контейнерами и деплоем

## HTTP Server и API

### Стандартная библиотека net/http

#### Базовый сервер
```go
package main

import (
    "fmt"
    "net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, World!")
}

func main() {
    http.HandleFunc("/", handler)
    http.ListenAndServe(":8080", nil)
}
```

#### Использование http.ServeMux
```go
func main() {
    mux := http.NewServeMux()
    
    mux.HandleFunc("/", homeHandler)
    mux.HandleFunc("/users", usersHandler)
    mux.HandleFunc("/users/", userHandler) // /users/123
    
    server := &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }
    
    server.ListenAndServe()
}
```

### Работа с контекстом

#### Контекст в обработчиках
```go
import "context"

func handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Использование контекста для отмены
    result, err := longOperation(ctx)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(result)
}

func longOperation(ctx context.Context) (string, error) {
    select {
    case <-ctx.Done():
        return "", ctx.Err()
    case <-time.After(5 * time.Second):
        return "done", nil
    }
}
```

#### Таймауты
```go
func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", handler)
    
    server := &http.Server{
        Addr:         ":8080",
        Handler:      mux,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  120 * time.Second,
    }
    
    server.ListenAndServe()
}
```

### Middleware

#### Базовый middleware
```go
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        next.ServeHTTP(w, r)
        
        log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
    })
}

func recoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("panic recovered: %v", err)
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}

// Применение middleware
func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", handler)
    
    handlerChain := recoveryMiddleware(loggingMiddleware(mux))
    
    http.ListenAndServe(":8080", handlerChain)
}
```

#### Middleware для CORS
```go
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

#### Middleware для аутентификации
```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        // Валидация токена
        userID, err := validateToken(token)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        // Добавление userID в контекст
        ctx := context.WithValue(r.Context(), "userID", userID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### JSON API

#### Парсинг JSON запроса
```go
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()
    
    // Валидация
    if req.Name == "" || req.Email == "" {
        http.Error(w, "Name and email are required", http.StatusBadRequest)
        return
    }
    
    // Создание пользователя
    user := createUser(req)
    
    // Отправка ответа
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}
```

#### JSON ответ с ошибками
```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message,omitempty"`
}

func sendError(w http.ResponseWriter, status int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(ErrorResponse{
        Error:   http.StatusText(status),
        Message: message,
    })
}

func handler(w http.ResponseWriter, r *http.Request) {
    if err := doSomething(); err != nil {
        sendError(w, http.StatusInternalServerError, err.Error())
        return
    }
    // ...
}
```

#### Пагинация
```go
type Pagination struct {
    Page  int `json:"page" query:"page"`
    Limit int `json:"limit" query:"limit"`
    Total int `json:"total"`
}

type PaginatedResponse struct {
    Data       interface{} `json:"data"`
    Pagination Pagination  `json:"pagination"`
}

func getUsersHandler(w http.ResponseWriter, r *http.Request) {
    page := 1
    limit := 10
    
    if p := r.URL.Query().Get("page"); p != "" {
        if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
            page = parsed
        }
    }
    
    if l := r.URL.Query().Get("limit"); l != "" {
        if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
            limit = parsed
        }
    }
    
    users, total := getUsers(page, limit)
    
    response := PaginatedResponse{
        Data: users,
        Pagination: Pagination{
            Page:  page,
            Limit: limit,
            Total: total,
        },
    }
    
    json.NewEncoder(w).Encode(response)
}
```

### Веб-фреймворки

#### Gin
```go
import "github.com/gin-gonic/gin"

func main() {
    r := gin.Default()
    
    // Middleware
    r.Use(gin.Logger())
    r.Use(gin.Recovery())
    
    // Routes
    r.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Hello"})
    })
    
    r.GET("/users/:id", getUserHandler)
    r.POST("/users", createUserHandler)
    
    // Group
    api := r.Group("/api")
    {
        api.GET("/users", getUsersHandler)
        api.POST("/users", createUserHandler)
    }
    
    r.Run(":8080")
}

func getUserHandler(c *gin.Context) {
    id := c.Param("id")
    user, err := getUser(id)
    if err != nil {
        c.JSON(404, gin.H{"error": "User not found"})
        return
    }
    c.JSON(200, user)
}
```

#### Chi
```go
import "github.com/go-chi/chi/v5"

func main() {
    r := chi.NewRouter()
    
    // Middleware
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Timeout(60 * time.Second))
    
    // Routes
    r.Get("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello"))
    })
    
    r.Route("/users", func(r chi.Router) {
        r.Get("/", getUsersHandler)
        r.Post("/", createUserHandler)
        r.Get("/{id}", getUserHandler)
    })
    
    http.ListenAndServe(":8080", r)
}
```

#### Echo
```go
import "github.com/labstack/echo/v4"

func main() {
    e := echo.New()
    
    // Middleware
    e.Use(middleware.Logger())
    e.Use(middleware.Recover())
    e.Use(middleware.CORS())
    
    // Routes
    e.GET("/", func(c echo.Context) error {
        return c.JSON(200, map[string]string{"message": "Hello"})
    })
    
    e.GET("/users/:id", getUserHandler)
    e.POST("/users", createUserHandler)
    
    e.Start(":8080")
}
```

## Базы данных

### PostgreSQL с pgx

#### Подключение
```go
import "github.com/jackc/pgx/v5/pgxpool"

var pool *pgxpool.Pool

func initDB(ctx context.Context) error {
    config, err := pgxpool.ParseConfig("postgres://user:pass@localhost/db")
    if err != nil {
        return err
    }
    
    pool, err = pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return err
    }
    
    return pool.Ping(ctx)
}
```

#### Выполнение запросов
```go
type User struct {
    ID    string
    Name  string
    Email string
}

func getUser(ctx context.Context, id string) (*User, error) {
    const query = `SELECT id, name, email FROM users WHERE id = $1`
    
    var user User
    err := pool.QueryRow(ctx, query, id).Scan(
        &user.ID,
        &user.Name,
        &user.Email,
    )
    
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, fmt.Errorf("user not found")
        }
        return nil, err
    }
    
    return &user, nil
}
```

#### Транзакции
```go
func createUserWithProfile(ctx context.Context, user User, profile Profile) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)
    
    // Создание пользователя
    if err := createUserTx(ctx, tx, user); err != nil {
        return err
    }
    
    // Создание профиля
    if err := createProfileTx(ctx, tx, profile); err != nil {
        return err
    }
    
    return tx.Commit(ctx)
}
```

#### Batch операции
```go
func batchInsertUsers(ctx context.Context, users []User) error {
    batch := &pgx.Batch{}
    
    const query = `INSERT INTO users (id, name, email) VALUES ($1, $2, $3)`
    
    for _, user := range users {
        batch.Queue(query, user.ID, user.Name, user.Email)
    }
    
    br := pool.SendBatch(ctx, batch)
    defer br.Close()
    
    for i := 0; i < len(users); i++ {
        _, err := br.Exec()
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### SQL с database/sql

#### Подключение
```go
import "database/sql"

var db *sql.DB

func initDB() error {
    var err error
    db, err = sql.Open("postgres", "postgres://user:pass@localhost/db")
    if err != nil {
        return err
    }
    
    return db.Ping()
}
```

#### Выполнение запросов
```go
func getUser(id string) (*User, error) {
    const query = `SELECT id, name, email FROM users WHERE id = $1`
    
    var user User
    err := db.QueryRow(query, id).Scan(&user.ID, &user.Name, &user.Email)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, fmt.Errorf("user not found")
        }
        return nil, err
    }
    
    return &user, nil
}

func listUsers() ([]User, error) {
    const query = `SELECT id, name, email FROM users ORDER BY name`
    
    rows, err := db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var users []User
    for rows.Next() {
        var user User
        if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
            return nil, err
        }
        users = append(users, user)
    }
    
    return users, rows.Err()
}
```

### GORM

#### Настройка
```go
import "gorm.io/gorm"
import "gorm.io/driver/postgres"

var db *gorm.DB

func initDB() error {
    var err error
    dsn := "host=localhost user=postgres password=pass dbname=mydb port=5432 sslmode=disable"
    db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
    return err
}
```

#### Модели
```go
type User struct {
    ID        string    `gorm:"primaryKey"`
    Name      string    `gorm:"not null"`
    Email     string    `gorm:"uniqueIndex;not null"`
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt `gorm:"index"`
}
```

#### CRUD операции
```go
// Create
user := User{ID: uuid.New().String(), Name: "John", Email: "john@example.com"}
if err := db.Create(&user).Error; err != nil {
    return err
}

// Read
var user User
if err := db.First(&user, "id = ?", id).Error; err != nil {
    return err
}

// Update
if err := db.Model(&user).Update("name", "Jane").Error; err != nil {
    return err
}

// Delete
if err := db.Delete(&user).Error; err != nil {
    return err
}

// List
var users []User
if err := db.Find(&users).Error; err != nil {
    return err
}
```

### Redis

#### Подключение и базовые операции
```go
import "github.com/redis/go-redis/v9"

var rdb *redis.Client

func initRedis() {
    rdb = redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
    })
}

func setCache(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
    return rdb.Set(ctx, key, value, expiration).Err()
}

func getCache(ctx context.Context, key string) (string, error) {
    return rdb.Get(ctx, key).Result()
}

func deleteCache(ctx context.Context, keys ...string) error {
    return rdb.Del(ctx, keys...).Err()
}
```

## Конфигурация

### Viper
```go
import "github.com/spf13/viper"

type Config struct {
    Server struct {
        Port int `mapstructure:"port"`
    } `mapstructure:"server"`
    
    Database struct {
        Host     string `mapstructure:"host"`
        Port     int    `mapstructure:"port"`
        User     string `mapstructure:"user"`
        Password string `mapstructure:"password"`
        DBName   string `mapstructure:"dbname"`
    } `mapstructure:"database"`
}

func loadConfig(path string) (*Config, error) {
    viper.SetConfigFile(path)
    viper.SetConfigType("yaml")
    
    viper.AutomaticEnv()
    viper.SetEnvPrefix("APP")
    
    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var config Config
    if err := viper.Unmarshal(&config); err != nil {
        return nil, err
    }
    
    return &config, nil
}
```

### Environment variables
```go
import "os"

type Config struct {
    Port        string
    DatabaseURL string
}

func loadConfigFromEnv() *Config {
    return &Config{
        Port:        getEnv("PORT", "8080"),
        DatabaseURL: getEnv("DATABASE_URL", ""),
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

## Логирование

### Logrus
```go
import "github.com/sirupsen/logrus"

var log *logrus.Logger

func initLogger() {
    log = logrus.New()
    log.SetFormatter(&logrus.JSONFormatter{})
    log.SetLevel(logrus.InfoLevel)
}

func main() {
    log.WithFields(logrus.Fields{
        "event": "user_created",
        "user_id": "123",
    }).Info("User created successfully")
}
```

### Zap
```go
import "go.uber.org/zap"

var logger *zap.Logger

func initLogger() error {
    var err error
    logger, err = zap.NewProduction()
    return err
}

func main() {
    logger.Info("User created",
        zap.String("user_id", "123"),
        zap.String("email", "user@example.com"),
    )
}
```

### Structured logging с контекстом
```go
func requestLogger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()
        }
        
        ctx := context.WithValue(r.Context(), "requestID", requestID)
        r = r.WithContext(ctx)
        
        next.ServeHTTP(w, r)
        
        logger.Info("Request completed",
            zap.String("method", r.Method),
            zap.String("path", r.URL.Path),
            zap.Duration("duration", time.Since(start)),
            zap.String("request_id", requestID),
        )
    })
}
```

## Аутентификация и авторизация

### JWT
```go
import "github.com/golang-jwt/jwt/v5"

type Claims struct {
    UserID string `json:"user_id"`
    jwt.RegisteredClaims
}

func generateToken(userID string, secret string) (string, error) {
    claims := Claims{
        UserID: userID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

func validateToken(tokenString, secret string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(secret), nil
    })
    
    if err != nil {
        return nil, err
    }
    
    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }
    
    return nil, fmt.Errorf("invalid token")
}
```

### Password hashing
```go
import "golang.org/x/crypto/bcrypt"

func hashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

func checkPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

## Микросервисы

### gRPC

#### Proto файл
```protobuf
syntax = "proto3";

package user;

service UserService {
    rpc GetUser(GetUserRequest) returns (GetUserResponse);
    rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
}

message GetUserRequest {
    string id = 1;
}

message GetUserResponse {
    User user = 1;
}

message User {
    string id = 1;
    string name = 2;
    string email = 3;
}
```

#### Сервер
```go
import "google.golang.org/grpc"

type server struct {
    pb.UnimplementedUserServiceServer
}

func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
    user, err := getUser(req.Id)
    if err != nil {
        return nil, status.Errorf(codes.NotFound, "user not found")
    }
    
    return &pb.GetUserResponse{
        User: &pb.User{
            Id:    user.ID,
            Name:  user.Name,
            Email: user.Email,
        },
    }, nil
}

func main() {
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatal(err)
    }
    
    s := grpc.NewServer()
    pb.RegisterUserServiceServer(s, &server{})
    
    if err := s.Serve(lis); err != nil {
        log.Fatal(err)
    }
}
```

#### Клиент
```go
func main() {
    conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    
    client := pb.NewUserServiceClient(conn)
    
    resp, err := client.GetUser(context.Background(), &pb.GetUserRequest{Id: "123"})
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("User: %+v\n", resp.User)
}
```

### Service Discovery с Consul
```go
import "github.com/hashicorp/consul/api"

func registerService() error {
    config := api.DefaultConfig()
    client, err := api.NewClient(config)
    if err != nil {
        return err
    }
    
    registration := &api.AgentServiceRegistration{
        ID:      "user-service-1",
        Name:    "user-service",
        Port:    8080,
        Address: "192.168.1.100",
        Check: &api.AgentServiceCheck{
            HTTP:                           "http://192.168.1.100:8080/health",
            Interval:                       "10s",
            Timeout:                        "5s",
            DeregisterCriticalServiceAfter: "30s",
        },
    }
    
    return client.Agent().ServiceRegister(registration)
}

func discoverService(serviceName string) ([]*api.AgentService, error) {
    config := api.DefaultConfig()
    client, err := api.NewClient(config)
    if err != nil {
        return nil, err
    }
    
    services, _, err := client.Health().Service(serviceName, "", true, nil)
    return services, err
}
```

## Docker и деплой

### Dockerfile
```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/app

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]
```

### docker-compose.yml
```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://user:pass@db:5432/mydb
      - REDIS_URL=redis://redis:6379
    depends_on:
      - db
      - redis
  
  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
      - POSTGRES_DB=mydb
    volumes:
      - postgres_data:/var/lib/postgresql/data
  
  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

## Инструменты разработки

### Air (hot reload)
```bash
# Install
go install github.com/cosmtrek/air@latest

# .air.toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/main ./cmd/app"
bin = "tmp/main"
include_ext = ["go", "tpl", "tmpl", "html"]
exclude_dir = ["tmp", "vendor"]
delay = 1000
stop_on_error = true

[log]
time = true
```

### golangci-lint
```bash
# Install
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run
golangci-lint run ./...

# .golangci.yml
linters:
  enable:
    - gofmt
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - structcheck
    - varcheck
    - ineffassign
    - deadcode

linters-settings:
  govet:
    check-shadowing: true
```

### Makefile
```makefile
.PHONY: build run test lint clean

build:
	go build -o bin/app ./cmd/app

run:
	go run ./cmd/app

test:
	go test -v -race -cover ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

migrate-up:
	migrate -path migrations -database "postgres://user:pass@localhost/db" up

migrate-down:
	migrate -path migrations -database "postgres://user:pass@localhost/db" down
```

## Чек-лист для backend-разработки

Перед завершением работы проверь:
- [ ] API соответствует REST принципам
- [ ] Обработка ошибок с правильными HTTP статусами
- [ ] Middleware для логирования, CORS, аутентификации
- [ ] Контекст используется для отмены операций
- [ ] Таймауты для всех внешних вызовов
- [ ] SQL инъекции защищены (параметризованные запросы)
- [ ] Пароли хешируются
- [ ] JWT токены с истечением срока
- [ ] Логирование структурированное и информативное
- [ ] Конфигурация через environment variables
- [ ] Health check endpoint
- [ ] Graceful shutdown
- [ ] Тесты покрывают основные сценарии
- [ ] Dockerfile оптимизирован (multi-stage build)
- [ ] CI/CD настроен

## Дополнительные ресурсы

- https://go.dev/doc/effective_go
- https://github.com/golang/go/wiki
- https://pkg.go.dev/
- https://github.com/golang-standards/project-layout

Эти инструкции supersede любые общие инструкции режима Code. Выполняй только то, что указано в этом скилле.
