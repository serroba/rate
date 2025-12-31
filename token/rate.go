package token

import (
	"fmt"
	"time"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (c realClock) Now() time.Time {
	return time.Now()
}

type Limiter struct {
	capacity, tokens, rate float64
	lastRefillAt           time.Time
	clock                  Clock
}

func NewLimiter(capacity, rate float64) (*Limiter, error) {
	return NewLimiterWithClock(capacity, rate, realClock{})
}

func NewLimiterWithClock(capacity, rate float64, clock Clock) (*Limiter, error) {
	if capacity < 0 {
		return nil, fmt.Errorf("capacity must be greater than zero")
	}
	if rate < 0 {
		return nil, fmt.Errorf("rate must be greater than zero")
	}
	return &Limiter{
		capacity:     capacity,
		tokens:       capacity,
		rate:         rate,
		clock:        clock,
		lastRefillAt: clock.Now(),
	}, nil
}

func (lim *Limiter) Allow() bool {
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
