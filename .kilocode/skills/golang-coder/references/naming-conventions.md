# Naming Conventions

Complete guide to Go naming conventions and visibility rules.

## Visibility

- **Exported** (public): Start with capital letter
- **Unexported** (private): Start with lowercase letter

```go
type User struct {
    ID       string    // exported field
    Name     string    // exported field
    password string    // private field (not exported)
}

func GetUser(id string) *User { ... }  // exported function
func validateUser(u *User) error { ... } // private function
```

## Package Names

- Short, lowercase, single word
- Avoid underscores and mixed case
- Package should describe its contents

```go
package user      // good
package users     // good
package userData  // bad (camelCase)
package user_data // bad (underscore)
```

## Interfaces

- Single-method interfaces: Name ends with -er
- Multi-method interfaces: Descriptive name

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

## Getters and Setters

- Getters: Field name without Get prefix
- Setters: Set + field name

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

## Constants with iota

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

## Best Practices

1. Use short, clear names for local variables
2. Use descriptive names for exported identifiers
3. Avoid redundant names (e.g., `user.UserName` → `user.Name`)
4. Use consistent naming across the codebase
5. Follow standard library naming patterns
