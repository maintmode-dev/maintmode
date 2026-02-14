# Concurrency

Goroutines, channels, and synchronization patterns in Go.

## Goroutines

```go
func main() {
    go func() {
        fmt.Println("goroutine")
    }()

    time.Sleep(time.Second) // only for example
}
```

## Channels

```go
// Creating
ch := make(chan int)          // unbuffered
ch := make(chan int, 10)      // buffered

// Sending (blocks if buffer full)
ch <- 42

// Receiving (blocks if empty)
v := <-ch

// Closing
close(ch)

// Checking if closed
v, ok := <-ch  // ok == false if channel closed

// Range over channel
for v := range ch {
    fmt.Println(v)
}
```

## Worker Pool Pattern

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

    // Start workers
    for w := 1; w <= 3; w++ {
        go worker(w, jobs, results)
    }

    // Send jobs
    for j := 1; j <= 5; j++ {
        jobs <- j
    }
    close(jobs)

    // Receive results
    for a := 1; a <= 5; a++ {
        <-results
    }
}
```

## Select

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

## Sync Primitives

### Mutex

```go
import "sync"

var mu sync.Mutex
var counter int

func increment() {
    mu.Lock()
    defer mu.Unlock()
    counter++
}
```

### WaitGroup

```go
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
```

### Once

```go
var once sync.Once
var config *Config

func getConfig() *Config {
    once.Do(func() {
        config = loadConfig()
    })
    return config
}
```

## Context

```go
import "context"

func doWork(ctx context.Context) error {
    select {
    case <-ctx.Done():
        return ctx.Err() // context.Canceled or context.DeadlineExceeded
    default:
        // work
    }
}
```

## Best Practices

1. **Don't start goroutines you can't stop** - Use context for cancellation
2. **Close channels from sender side** - Receivers don't close
3. **Use buffered channels for known capacity** - Avoid blocking
4. **Avoid sharing memory by communicating** - Use channels
5. **Use sync primitives when channels aren't suitable** - Direct synchronization
6. **Always use WaitGroup.Wait()** - Ensure all goroutines finish
7. **Test with -race flag** - Detect data races
8. **Use context for cancellation and timeouts** - Clean shutdown
