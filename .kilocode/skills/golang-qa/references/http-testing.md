# HTTP Testing

Complete guide to testing HTTP handlers, APIs, and middleware in Go.

## Testing HTTP Handlers

### Basic Handler Test

```go
import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestUserHandler(t *testing.T) {
    req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
    rec := httptest.NewRecorder()

    UserHandler(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
    assert.Contains(t, rec.Body.String(), "John")
}
```

### Testing JSON APIs

```go
func TestCreateUserHandler(t *testing.T) {
    payload := `{"name":"John","email":"john@example.com"}`
    req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(payload))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()

    CreateUserHandler(rec, req)

    assert.Equal(t, http.StatusCreated, rec.Code)

    var response map[string]interface{}
    err := json.NewDecoder(rec.Body).Decode(&response)
    assert.NoError(t, err)
    assert.Equal(t, "John", response["name"])
}
```

## Table-Driven HTTP Tests

```go
func TestUserAPI(t *testing.T) {
    tests := []struct {
        name           string
        method         string
        path           string
        body           string
        expectedStatus int
        expectedBody   string
    }{
        {
            name:           "get user success",
            method:         http.MethodGet,
            path:           "/users/123",
            expectedStatus: http.StatusOK,
            expectedBody:   `"name":"John"`,
        },
        {
            name:           "create user success",
            method:         http.MethodPost,
            path:           "/users",
            body:           `{"name":"Jane","email":"jane@example.com"}`,
            expectedStatus: http.StatusCreated,
            expectedBody:   `"name":"Jane"`,
        },
        {
            name:           "invalid payload",
            method:         http.MethodPost,
            path:           "/users",
            body:           `invalid json`,
            expectedStatus: http.StatusBadRequest,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var body io.Reader
            if tt.body != "" {
                body = strings.NewReader(tt.body)
            }

            req := httptest.NewRequest(tt.method, tt.path, body)
            if tt.body != "" {
                req.Header.Set("Content-Type", "application/json")
            }
            rec := httptest.NewRecorder()

            // Call handler
            router.ServeHTTP(rec, req)

            assert.Equal(t, tt.expectedStatus, rec.Code)
            if tt.expectedBody != "" {
                assert.Contains(t, rec.Body.String(), tt.expectedBody)
            }
        })
    }
}
```

## Testing with Test Server

```go
func TestAPIClient(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "/users/123", r.URL.Path)
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{
            "id":   "123",
            "name": "John",
        })
    }))
    defer server.Close()

    // Test client
    client := NewAPIClient(server.URL)
    user, err := client.GetUser(123)

    assert.NoError(t, err)
    assert.Equal(t, "John", user.Name)
}
```

## Testing Middleware

```go
func TestAuthMiddleware(t *testing.T) {
    tests := []struct {
        name           string
        token          string
        expectedStatus int
    }{
        {
            name:           "valid token",
            token:          "Bearer valid-token",
            expectedStatus: http.StatusOK,
        },
        {
            name:           "missing token",
            token:          "",
            expectedStatus: http.StatusUnauthorized,
        },
        {
            name:           "invalid token",
            token:          "Bearer invalid",
            expectedStatus: http.StatusUnauthorized,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Mock handler
            nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
            })

            // Apply middleware
            handler := AuthMiddleware(nextHandler)

            req := httptest.NewRequest(http.MethodGet, "/protected", nil)
            if tt.token != "" {
                req.Header.Set("Authorization", tt.token)
            }
            rec := httptest.NewRecorder()

            handler.ServeHTTP(rec, req)

            assert.Equal(t, tt.expectedStatus, rec.Code)
        })
    }
}
```

## Testing with Echo Framework

```go
import (
    "github.com/labstack/echo/v4"
)

func TestEchoHandler(t *testing.T) {
    e := echo.New()
    req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    c.SetParamNames("id")
    c.SetParamValues("123")

    err := GetUserHandler(c)

    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, rec.Code)
}
```

## Testing File Uploads

```go
func TestFileUploadHandler(t *testing.T) {
    // Create multipart form
    body := new(bytes.Buffer)
    writer := multipart.NewWriter(body)

    // Add file
    part, err := writer.CreateFormFile("file", "test.txt")
    require.NoError(t, err)
    _, err = part.Write([]byte("test content"))
    require.NoError(t, err)

    // Add form fields
    writer.WriteField("name", "test file")
    writer.Close()

    // Create request
    req := httptest.NewRequest(http.MethodPost, "/upload", body)
    req.Header.Set("Content-Type", writer.FormDataContentType())
    rec := httptest.NewRecorder()

    UploadHandler(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
}
```

## Testing Cookies and Sessions

```go
func TestSessionHandler(t *testing.T) {
    req := httptest.NewRequest(http.MethodGet, "/profile", nil)
    req.AddCookie(&http.Cookie{
        Name:  "session_id",
        Value: "valid-session",
    })
    rec := httptest.NewRecorder()

    ProfileHandler(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)

    // Check response cookies
    cookies := rec.Result().Cookies()
    assert.NotEmpty(t, cookies)
}
```

## Testing Response Headers

```go
func TestCORSHeaders(t *testing.T) {
    req := httptest.NewRequest(http.MethodOptions, "/api/users", nil)
    req.Header.Set("Origin", "https://example.com")
    rec := httptest.NewRecorder()

    CORSHandler(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
    assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
    assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")
}
```

## Testing Error Responses

```go
func TestErrorHandler(t *testing.T) {
    tests := []struct {
        name           string
        userID         string
        expectedStatus int
        expectedError  string
    }{
        {
            name:           "user not found",
            userID:         "999",
            expectedStatus: http.StatusNotFound,
            expectedError:  "user not found",
        },
        {
            name:           "invalid user id",
            userID:         "invalid",
            expectedStatus: http.StatusBadRequest,
            expectedError:  "invalid user id",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest(http.MethodGet, "/users/"+tt.userID, nil)
            rec := httptest.NewRecorder()

            GetUserHandler(rec, req)

            assert.Equal(t, tt.expectedStatus, rec.Code)

            var response map[string]string
            json.NewDecoder(rec.Body).Decode(&response)
            assert.Contains(t, response["error"], tt.expectedError)
        })
    }
}
```

## Best Practices

1. **Use httptest package** - Built-in testing support
2. **Test all HTTP methods** - GET, POST, PUT, DELETE, etc.
3. **Test status codes** - Verify correct response codes
4. **Test response bodies** - Check JSON structure and content
5. **Test headers and cookies** - Verify all response metadata
6. **Test error cases** - Invalid input, missing parameters, etc.
7. **Use table-driven tests** - Cover multiple scenarios
8. **Test middleware separately** - Isolate middleware logic
9. **Mock dependencies** - Don't hit real databases/APIs
10. **Test concurrent requests** - Verify thread safety

## Running HTTP Tests

```bash
# Run all tests
go test ./...

# Run specific handler test
go test -run TestUserHandler

# Run with verbose output
go test -v ./...

# Generate coverage
go test -cover ./...
```
