package window

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

type FixedLimiter struct {
	mu sync.Mutex

	limit, count uint32
	window       time.Duration
	start        time.Time
	clock        clock
}

// NewFixedLimiter creates a new fixed window rate limiter.
// Limit is the maximum requests per window. Window is the duration of each window.
func NewFixedLimiter(limit uint32, window time.Duration) *FixedLimiter {
	return NewFixedLimiterWithClock(limit, window, realClock{})
}

// NewFixedLimiterWithClock creates a new fixed window limiter with a custom clock.
// Use this constructor for testing with a mock clock.
func NewFixedLimiterWithClock(limit uint32, window time.Duration, clock clock) *FixedLimiter {
	if window == 0*time.Second {
		window = 1 * time.Second
	}

	return &FixedLimiter{
		limit:  limit,
		window: window,
		clock:  clock,
		start:  windowStart(clock.Now(), window),
	}
}

func windowStart(now time.Time, window time.Duration) time.Time {
	ns := now.UnixNano()
	w := window.Nanoseconds()

	return time.Unix(0, (ns/w)*w).UTC()
}

// Allow reports whether a request is allowed within the current window.
// Returns true if under the limit, false otherwise.
func (l *FixedLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	ws := windowStart(now, l.window)

	if !ws.Equal(l.start) {
		l.start = ws
		l.count = 0
	}

	if l.count+1 <= l.limit {
		l.count++

		return true
	}

	return false
}
