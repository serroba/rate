package window

import (
	"sync"
	"time"
)

// SlidingLimiter implements a sliding window rate limiter. It tracks individual
// request timestamps and allows requests if fewer than limit occurred in the
// last window duration. Unlike FixedLimiter, old requests expire individually.
type SlidingLimiter struct {
	mu     sync.Mutex
	window time.Duration
	limit  uint32
	q      []time.Time
	head   int
	clock  clock
}

// NewSlidingLimiter creates a new sliding window rate limiter.
// Limit is the maximum requests per window. Duration is the sliding window size.
func NewSlidingLimiter(limit uint32, duration time.Duration) *SlidingLimiter {
	return NewSlidingLimiterWithClock(limit, duration, realClock{})
}

// NewSlidingLimiterWithClock creates a new sliding window limiter with a custom clock.
// Use this constructor for testing with a mock clock.
func NewSlidingLimiterWithClock(limit uint32, duration time.Duration, clock clock) *SlidingLimiter {
	if duration == 0 {
		duration = 1 * time.Second
	}

	return &SlidingLimiter{
		window: duration,
		limit:  limit,
		q:      make([]time.Time, 0),
		clock:  clock,
	}
}

// Allow reports whether a request is allowed within the sliding window.
// Returns true if under the limit, false otherwise.
func (l *SlidingLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	cutoff := now.Add(-l.window)

	for l.head < len(l.q) && l.q[l.head].Before(cutoff) {
		l.head++
	}

	if l.head > 0 && l.head*2 >= len(l.q) {
		l.q = append([]time.Time(nil), l.q[l.head:]...)
		l.head = 0
	}

	if len(l.q)-l.head+1 > int(l.limit) {
		return false
	}

	l.q = append(l.q, now)

	return true
}
