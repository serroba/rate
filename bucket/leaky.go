package bucket

import (
	"sync"
	"time"
)

// LeakyLimiter implements a leaky bucket rate limiter. Requests fill the bucket,
// which drains at a constant rate. If the bucket is full, requests are rejected.
// Unlike TokenLimiter, this enforces a smooth output rate rather than allowing bursts.
type LeakyLimiter struct {
	mu sync.Mutex

	capacity, level, rate float64
	lastUpdatedAt         time.Time
	clock                 clock
}

// NewLeakyLimiter creates a new leaky bucket limiter.
// Capacity is the maximum bucket size. Rate is how many requests drain per second.
func NewLeakyLimiter(capacity, rate uint32) *LeakyLimiter {
	return NewLeakyLimiterWithClock(capacity, rate, realClock{})
}

// NewLeakyLimiterWithClock creates a new leaky bucket limiter with a custom clock.
// Use this constructor for testing with a mock clock.
func NewLeakyLimiterWithClock(capacity, rate uint32, clock clock) *LeakyLimiter {
	return &LeakyLimiter{
		capacity:      float64(capacity),
		rate:          float64(rate),
		clock:         clock,
		lastUpdatedAt: clock.Now(),
	}
}

func (lim *LeakyLimiter) update() {
	t := lim.clock.Now()
	if t.Before(lim.lastUpdatedAt) {
		return
	}

	lim.level = max(0, lim.level-t.Sub(lim.lastUpdatedAt).Seconds()*lim.rate)
	lim.lastUpdatedAt = t
}

// Allow reports whether a request is allowed. It adds one to the bucket level
// if there is room and returns true. If the bucket is full, it returns false
// without blocking.
func (lim *LeakyLimiter) Allow() bool {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	lim.update()

	if lim.level+1 <= lim.capacity {
		lim.level++

		return true
	}

	return false
}
