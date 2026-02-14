# Table-Driven Tests

Table-driven tests are a Go testing pattern where test cases are defined as data structures in a table (slice), allowing multiple test scenarios to be executed with the same test logic.

## Basic Pattern

```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {
            name:     "valid input",
            input:    "test",
            expected: "TEST",
            wantErr:  false,
        },
        {
            name:     "empty input",
            input:    "",
            expected: "",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := ToUpper(tt.input)

            if tt.wantErr {
                require.Error(t, err)
                return
            }

            require.NoError(t, err)
            require.Equal(t, tt.expected, result)
        })
    }
}
```

## Parallel Execution

For independent tests, use `t.Parallel()` to run test cases concurrently:

```go
func TestFunction(t *testing.T) {
    t.Parallel() // Parent test runs in parallel with other tests

    tests := []struct {
        name     string
        input    int
        expected int
    }{
        {"positive", 5, 25},
        {"negative", -3, 9},
        {"zero", 0, 0},
    }

    for _, tt := range tests {
        tt := tt // Capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // Each subtest runs in parallel

            result := Square(tt.input)
            require.Equal(t, tt.expected, result)
        })
    }
}
```

**Important:** When using `t.Parallel()`, capture the loop variable with `tt := tt` to avoid race conditions.

## Complex Test Cases

For complex scenarios, include setup and expected error details:

```go
func TestCreateMaintenance(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    now := time.Now().UTC()

    tests := []struct {
        name        string
        maint       *entity.Maintenance
        expectedErr string
        validate    func(t *testing.T, result *entity.Maintenance)
    }{
        {
            name: "valid planned period",
            maint: &entity.Maintenance{
                ID:            uuid.New(),
                Title:         "Test Maintenance",
                PlannedPeriod: entity.NewPeriod(now, now.Add(time.Hour)),
                Status:        entity.StatusPlanned,
            },
            expectedErr: "",
            validate: func(t *testing.T, result *entity.Maintenance) {
                require.NotNil(t, result)
                require.Equal(t, "Test Maintenance", result.Title)
            },
        },
        {
            name: "invalid period - start equals end",
            maint: &entity.Maintenance{
                ID:            uuid.New(),
                Title:         "Invalid",
                PlannedPeriod: entity.NewPeriod(now, now),
                Status:        entity.StatusPlanned,
            },
            expectedErr: "pq: new row violates check constraint",
            validate:    nil,
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            err := store.Create(ctx, tt.maint)

            if tt.expectedErr != "" {
                require.ErrorContains(t, err, tt.expectedErr)
                return
            }

            require.NoError(t, err)
            if tt.validate != nil {
                tt.validate(t, tt.maint)
            }
        })
    }
}
```

## Table-Driven Tests with Setup/Teardown

When tests need setup or cleanup, use a setup function:

```go
func TestWithSetup(t *testing.T) {
    tests := []struct {
        name     string
        setup    func(t *testing.T) *User
        userID   string
        expected *User
    }{
        {
            name: "existing user",
            setup: func(t *testing.T) *User {
                user := &User{ID: "123", Name: "John"}
                require.NoError(t, db.Create(user))
                return user
            },
            userID: "123",
        },
        {
            name: "non-existing user",
            setup: func(t *testing.T) *User {
                return nil
            },
            userID:   "999",
            expected: nil,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            expected := tt.setup(t)

            result, err := service.GetUser(tt.userID)

            if expected == nil {
                require.Error(t, err)
                return
            }

            require.NoError(t, err)
            require.Equal(t, expected, result)
        })
    }
}
```

## Nested Table-Driven Tests

For testing multiple dimensions, nest table-driven tests:

```go
func TestValidation(t *testing.T) {
    validationTests := []struct {
        name  string
        field string
        tests []struct {
            name    string
            value   interface{}
            wantErr bool
        }
    }{
        {
            name:  "email validation",
            field: "email",
            tests: []struct {
                name    string
                value   interface{}
                wantErr bool
            }{
                {"valid email", "user@example.com", false},
                {"invalid format", "not-an-email", true},
                {"empty", "", true},
            },
        },
        {
            name:  "age validation",
            field: "age",
            tests: []struct {
                name    string
                value   interface{}
                wantErr bool
            }{
                {"valid age", 25, false},
                {"negative", -5, true},
                {"too old", 200, true},
            },
        },
    }

    for _, vt := range validationTests {
        t.Run(vt.name, func(t *testing.T) {
            for _, tt := range vt.tests {
                t.Run(tt.name, func(t *testing.T) {
                    err := validate(vt.field, tt.value)
                    if tt.wantErr {
                        require.Error(t, err)
                    } else {
                        require.NoError(t, err)
                    }
                })
            }
        })
    }
}
```

## Best Practices

1. **Use descriptive test names** - Each test case should have a clear `name` field
2. **Capture loop variables** - Always use `tt := tt` when using `t.Parallel()`
3. **Keep test cases independent** - Each test should be able to run in isolation
4. **Use helper functions** - Extract common setup/validation logic
5. **Test edge cases** - Include boundary values, empty inputs, and error scenarios
6. **Order tests logically** - Group related tests and order from simple to complex
7. **Use subtests** - Always use `t.Run()` for better test organization and reporting
