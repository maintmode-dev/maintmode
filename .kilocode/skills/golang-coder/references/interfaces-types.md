# Interfaces and Types

Working with interfaces, type assertions, collections, and composition in Go.

## Implicit Interface Implementation

```go
type Writer interface {
    Write([]byte) (int, error)
}

// File implements Writer automatically
type File struct {
    // ...
}

func (f *File) Write(p []byte) (int, error) {
    // implementation
}
```

## Type Assertion

```go
var i interface{} = "hello"

s := i.(string)  // panic if not string
s, ok := i.(string)  // safe check
```

## Type Switch

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

## Slices

```go
// Creating slices
s := []int{1, 2, 3}           // literal
s := make([]int, 0, 10)       // with capacity
s := make([]int, 5)           // with length

// Appending elements
s = append(s, 4)               // append returns new slice

// Iteration
for i, v := range s {
    fmt.Printf("%d: %d\n", i, v)
}

// Sub-slice (creates reference to same array)
sub := s[1:3]

// Copying (independent slice)
copy := make([]int, len(s))
copy(copy, s)
```

## Maps

```go
// Creating
m := make(map[string]int)
m := map[string]int{"a": 1, "b": 2}

// Reading with existence check
if v, ok := m["key"]; ok {
    fmt.Println(v)
}

// Deleting
delete(m, "key")

// Iteration (order not guaranteed)
for k, v := range m {
    fmt.Printf("%s: %d\n", k, v)
}
```

## Embedding

```go
type Reader struct {
    io.Reader
    // additional fields
}

type Base struct {
    ID string
}

type Extended struct {
    Base  // embedding - all Base methods and fields accessible
    Name  string
}

func (e *Extended) Method() {
    fmt.Println(e.ID) // access to Base field
}
```

## Best Practices

1. **Keep interfaces small** - Single responsibility
2. **Accept interfaces, return structs** - More flexible APIs
3. **Use type assertions sparingly** - Prefer interface methods
4. **Pre-allocate slices when size is known** - Better performance
5. **Don't mutate maps during iteration** - Undefined behavior
6. **Use embedding for composition** - Not inheritance
