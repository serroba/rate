package token

import "time"

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

func NewLimiter(capacity, rate float64) *Limiter {
	clock := realClock{}

	return &Limiter{
		capacity:     capacity,
		tokens:       capacity,
		rate:         rate,
		clock:        clock,
		lastRefillAt: clock.Now(),
	}
}

func NewLimiterWithClock(capacity, rate float64, clock Clock) *Limiter {
	return &Limiter{
		capacity:     capacity,
		tokens:       capacity,
		rate:         rate,
		clock:        clock,
		lastRefillAt: clock.Now(),
	}
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
