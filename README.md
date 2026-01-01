# rate - Go Rate Limiting Library

[![CI](https://github.com/serroba/rate/actions/workflows/ci.yml/badge.svg)](https://github.com/serroba/rate/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/serroba/rate/branch/main/graph/badge.svg)](https://codecov.io/gh/serroba/rate)
[![Go Report Card](https://goreportcard.com/badge/github.com/serroba/rate)](https://goreportcard.com/report/github.com/serroba/rate)
[![Go Reference](https://pkg.go.dev/badge/github.com/serroba/rate.svg)](https://pkg.go.dev/github.com/serroba/rate)

A rate limiting library and web middleware for Go with multiple algorithm implementations. Thread-safe, zero dependencies (beyond testing), and designed for high-throughput applications.

## Features

- **Multiple Algorithms** - Token Bucket, Leaky Bucket, Fixed Window, Sliding Window, and GCRA
- **HTTP Middleware** - Ready-to-use middleware for `net/http`
- **Thread-Safe** - All implementations are safe for concurrent use
- **Zero Dependencies** - No external dependencies for production use
- **Registry Support** - Built-in per-key rate limiting (e.g., per-user, per-IP)
- **Testable** - Clock injection for deterministic unit tests
- **High Performance** - Lock-free where possible, minimal allocations

## Installation

```bash
go get github.com/serroba/rate
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/serroba/rate/bucket"
)

func main() {
    // Create a limiter: 10 requests/second, burst of 5
    lim := bucket.NewTokenLimiter(5, 10)

    for i := 0; i < 10; i++ {
        if lim.Allow() {
            fmt.Println("Request allowed")
        } else {
            fmt.Println("Request denied")
        }
    }
}
```

## Algorithms

| Algorithm      | Package                    | Best For                                       |
|----------------|----------------------------|------------------------------------------------|
| Token Bucket   | `bucket.NewTokenLimiter`   | Smooth rate limiting with burst tolerance      |
| Leaky Bucket   | `bucket.NewLeakyLimiter`   | Constant output rate, queuing semantics        |
| Fixed Window   | `window.NewFixedLimiter`   | Simple time-based quotas                       |
| Sliding Window | `window.NewSlidingLimiter` | Accurate rate limiting without boundary issues |
| GCRA           | `bucket.NewGCRALimiter`    | Memory-efficient, single timestamp approach    |

### Token Bucket

Allows bursts up to capacity, then refills at a steady rate.

```go
// 100 capacity, refills 10 tokens per second
lim := bucket.NewTokenLimiter(100, 10)
```

### Leaky Bucket

Requests fill the bucket; it drains at a constant rate.

```go
// Holds 100 requests, drains 10 per second
lim := bucket.NewLeakyLimiter(100, 10)
```

### Fixed Window

Simple counter that resets at fixed intervals.

```go
// 100 requests per minute
lim := window.NewFixedLimiter(100, time.Minute)
```

### Sliding Window

Tracks individual request timestamps for accurate rate limiting.

```go
// 100 requests per minute (sliding)
lim := window.NewSlidingLimiter(100, time.Minute)
```

### GCRA (Generic Cell Rate Algorithm)

Memory-efficient algorithm using a single timestamp. Originally designed for ATM networks.

```go
// 10 requests/second with burst of 5
lim := bucket.NewGCRALimiter(10, 5)
```

## Per-Key Rate Limiting

Use the Registry to manage rate limiters per identifier (user ID, IP address, API key, etc.):

```go
import (
    "github.com/serroba/rate/bucket"
    "github.com/serroba/rate/registry"
)

// Create a registry with a factory function
reg, _ := registry.NewRegistry(func() registry.Limiter {
    return bucket.NewTokenLimiter(100, 10)
})

// Rate limit per user
if reg.Allow("user-123") {
    // Handle request
}

if reg.Allow("user-456") {
    // Different user, different bucket
}
```

## HTTP Middleware

Ready-to-use middleware for `net/http`:

```go
import (
    "net/http"
    "github.com/serroba/rate/bucket"
    "github.com/serroba/rate/middleware"
    "github.com/serroba/rate/registry"
)

func main() {
    // Create a registry with your preferred algorithm
    reg, _ := registry.NewRegistry(func() registry.Limiter {
        return bucket.NewTokenLimiter(100, 10) // 100 burst, 10/sec refill
    })

    // Wrap your handler with rate limiting (uses client IP by default)
    handler := middleware.RateLimiter(reg, nil)(yourHandler)

    http.ListenAndServe(":8080", handler)
}
```

### Custom Key Extraction

Rate limit by API key, user ID, or any request attribute:

```go
// Rate limit by API key header
keyFunc := middleware.HeaderKeyFunc("X-Api-Key")
handler := middleware.RateLimiter(reg, keyFunc)(yourHandler)

// Or implement your own KeyFunc
customKeyFunc := func(r *http.Request) registry.Identifier {
    return registry.Identifier(r.URL.Path) // Rate limit per endpoint
}
```

### Key Extractors

| Function              | Description                                                        |
|-----------------------|--------------------------------------------------------------------|
| `IPKeyFunc`           | Extracts client IP (checks X-Forwarded-For, X-Real-IP, RemoteAddr) |
| `HeaderKeyFunc(name)` | Extracts value from specified header                               |

## Testing

All limiters support clock injection for deterministic tests:

```go
type mockClock struct {
    now time.Time
}

func (c *mockClock) Now() time.Time {
    return c.now
}

func (c *mockClock) Advance(d time.Duration) {
    c.now = c.now.Add(d)
}

func TestRateLimiting(t *testing.T) {
    clock := &mockClock{now: time.Now()}
    lim := bucket.NewLimiterWithClock(2, 1, clock)

    // Use all tokens
    require.True(t, lim.Allow())
    require.True(t, lim.Allow())
    require.False(t, lim.Allow())

    // Advance time to refill
    clock.Advance(time.Second)
    require.True(t, lim.Allow())
}
```

## Development

```bash
# Run tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run tests with coverage
go test ./... -coverprofile=coverage.out

# Run linter
golangci-lint run
```

## License

MIT License - see [LICENSE](LICENSE) for details.
