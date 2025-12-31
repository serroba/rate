package token

import (
	"errors"
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

// Limiter implements a token bucket rate limiter. It allows a burst of
// requests up to capacity, then refills tokens at the specified rate per second.
type Limiter struct {
	mu                     sync.Mutex
	capacity, tokens, rate float64
	lastRefillAt           time.Time
	clock                  clock
}

// NewLimiter creates a new rate limiter with the given capacity and refill rate.
// Capacity is the maximum burst size. Rate is tokens added per second.
// Returns an error if capacity or rate is negative.
func NewLimiter(capacity, rate float64) (*Limiter, error) {
	return NewLimiterWithClock(capacity, rate, realClock{})
}

// NewLimiterWithClock creates a new rate limiter with a custom clock.
// Use this constructor for testing with a mock clock.
func NewLimiterWithClock(capacity, rate float64, clock clock) (*Limiter, error) {
	if capacity < 0 {
		return nil, errors.New("capacity must be greater than zero")
	}

	if rate < 0 {
		return nil, errors.New("rate must be greater than zero")
	}

	return &Limiter{
		capacity:     capacity,
		tokens:       capacity,
		rate:         rate,
		clock:        clock,
		lastRefillAt: clock.Now(),
	}, nil
}

// Allow reports whether a request is allowed. It consumes one token if
// available and returns true. If no tokens are available, it returns false
// without blocking.
func (lim *Limiter) Allow() bool {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	lim.refill()

	if lim.tokens >= 1 {
		lim.tokens--

		return true
	}

	return false
}

func (lim *Limiter) refill() {
	t := lim.clock.Now()
	if t.Before(lim.lastRefillAt) {
		return
	}

	lim.tokens = min(lim.capacity, lim.tokens+t.Sub(lim.lastRefillAt).Seconds()*lim.rate)
	lim.lastRefillAt = t
}
