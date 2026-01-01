package bucket

import (
	"sync"
	"time"
)

type clock interface {
	Now() time.Time
}

type realClock struct{}

func (c realClock) Now() time.Time {
	return time.Now()
}

// TokenLimiter implements a bucket rate limiter. It allows a burst of
// requests up to capacity, then refills tokens at the specified rate per second.
type TokenLimiter struct {
	mu                     sync.Mutex
	capacity, tokens, rate float64
	lastRefillAt           time.Time
	clock                  clock
}

// NewTokenLimiter creates a new rate limiter with the given capacity and refill rate.
// Capacity is the maximum burst size. Rate is tokens added per second.
func NewTokenLimiter(capacity, rate uint32) *TokenLimiter {
	return NewLimiterWithClock(capacity, rate, realClock{})
}

// NewLimiterWithClock creates a new rate limiter with a custom clock.
// Use this constructor for testing with a mock clock.
func NewLimiterWithClock(capacity, rate uint32, clock clock) *TokenLimiter {
	return &TokenLimiter{
		capacity:     float64(capacity),
		tokens:       float64(capacity),
		rate:         float64(rate),
		clock:        clock,
		lastRefillAt: clock.Now(),
	}
}

// Allow reports whether a request is allowed. It consumes one bucket if
// available and returns true. If no tokens are available, it returns false
// without blocking.
func (lim *TokenLimiter) Allow() bool {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	lim.refill()

	if lim.tokens >= 1 {
		lim.tokens--

		return true
	}

	return false
}

func (lim *TokenLimiter) refill() {
	t := lim.clock.Now()
	if t.Before(lim.lastRefillAt) {
		return
	}

	lim.tokens = min(lim.capacity, lim.tokens+t.Sub(lim.lastRefillAt).Seconds()*lim.rate)
	lim.lastRefillAt = t
}
