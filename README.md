# Gemini Rate Limiter

[![Go
Reference](https://pkg.go.dev/badge/github.com/resoul/ratelimiter.svg)](https://pkg.go.dev/github.com/resoul/ratelimiter)
[![Go Report
Card](https://goreportcard.com/badge/github.com/resoul/ratelimiter)](https://goreportcard.com/report/github.com/resoul/ratelimiter)

Redis-based rate limiter for managing limits of the Gemini API (and
other services).

## Features

-   ⚡ Requests Per Minute limiting
-   🎯 Tokens Per Minute limiting
-   📅 Requests Per Day limiting
-   🔒 Atomic operations via Redis Pipeline
-   ⏰ Automatic key expiration
-   🧵 Thread-safe
-   🎨 Simple and clear API

## Installation

``` bash
go get github.com/resoul/ratelimiter
```

## Quick Start

``` go
package main

import (
    "context"
    "log"

    "github.com/redis/go-redis/v9"
    "github.com/resoul/ratelimiter"
)

func main() {
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    defer redisClient.Close()

    limiter := ratelimiter.New(redisClient, ratelimiter.Config{
        RequestsPerMinute: 15,
        TokensPerMinute:   250000,
        RequestsPerDay:    1000,
    })

    ctx := context.Background()

    result, err := limiter.CheckAndIncrement(ctx, 1500)
    if err != nil {
        log.Fatal(err)
    }

    if !result.Allowed {
        log.Printf("❌ Request blocked: %s", result.RejectionReason)
        log.Printf("⏰ Reset in: %s", result.ResetMinute)
        return
    }

    log.Println("✅ Request allowed!")
    log.Printf("📊 Current usage: %d/%d requests, %d/%d tokens", 
        result.CurrentRequests, 15,
        result.CurrentTokens, 250000)
}
```

## Usage

### Basic Example

``` go
limiter := ratelimiter.New(redisClient, ratelimiter.Config{
    RequestsPerMinute: 60,
    TokensPerMinute:   100000,
    RequestsPerDay:    10000,
})

result, err := limiter.CheckAndIncrement(ctx, estimatedTokens)
if err != nil {
    return err
}

if !result.Allowed {
    return fmt.Errorf("rate limit: %s", result.RejectionReason)
}
```

### Checking Current Usage

``` go
usage, err := limiter.GetCurrentUsage(ctx)
if err != nil {
    log.Fatal(err)
}

log.Printf("Requests this minute: %d", usage.CurrentRequests)
log.Printf("Tokens this minute: %d", usage.CurrentTokens)
log.Printf("Requests today: %d", usage.CurrentDayReqs)
log.Printf("Resets in: %s", usage.ResetMinute)
```

### Queue Integration

``` go
func processMessage(msg Message) error {
    estimatedTokens := int32(len(msg.Prompt) / 4)

    result, err := limiter.CheckAndIncrement(ctx, estimatedTokens)
    if err != nil {
        return err
    }

    if !result.Allowed {
        log.Printf("Rate limit hit, waiting %s", result.ResetMinute)
        time.Sleep(result.ResetMinute)
        return errRateLimitExceeded
    }

    return processWithAPI(msg)
}
```

### Graceful Degradation

``` go
result, err := limiter.CheckAndIncrement(ctx, tokens)
if err != nil {
    return fmt.Errorf("rate limiter unavailable: %w", err)
}

if !result.Allowed {
    return &RateLimitError{
        Reason:    result.RejectionReason,
        RetryAfter: result.ResetMinute,
    }
}
```

## API Reference

### Config

``` go
type Config struct {
    RequestsPerMinute int
    TokensPerMinute   int
    RequestsPerDay    int
}
```

### CheckResult

``` go
type CheckResult struct {
    Allowed           bool
    CurrentRequests   int
    CurrentTokens     int
    CurrentDayReqs    int
    ResetMinute       time.Duration
    ResetDay          time.Duration
    RejectionReason   string
}
```

### Methods

#### New

``` go
func New(redisClient *redis.Client, config Config) *RateLimiter
```

#### CheckAndIncrement

``` go
func (rl *RateLimiter) CheckAndIncrement(ctx context.Context, tokens int32) (*CheckResult, error)
```

#### GetCurrentUsage

``` go
func (rl *RateLimiter) GetCurrentUsage(ctx context.Context) (*CheckResult, error)
```

#### Reset

``` go
func (rl *RateLimiter) Reset(ctx context.Context) error
```

## Redis Keys

The library uses:

-   `gemini:ratelimit:minute:2024-11-25:15:30`
-   `gemini:ratelimit:tokens:minute:2024-11-25:15:30`
-   `gemini:ratelimit:day:2024-11-25`

## Testing

``` bash
docker run -d -p 6379:6379 redis:alpine

go test -v

go test -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Examples

More examples in [examples/](examples/).

## Requirements

-   Go 1.21+
-   Redis 6.0+

## License

MIT License --- see [LICENSE](LICENSE)

## Contributing

Pull requests are welcome! For major changes, open an issue first.
