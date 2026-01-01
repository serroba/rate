package bucket

import (
	"sync"
	"time"
)

// GCRALimiter implements the Generic Cell Rate Algorithm.
// It uses a single timestamp (Theoretical Arrival Time) instead of counters,
// making it extremely memory efficient. Originally designed for ATM networks.
//
// The algorithm works by tracking when the next request "should" arrive.
// If a request arrives too early, it's rejected. Burst tolerance allows
// some requests to arrive early, accumulating "credit" during idle periods.
type GCRALimiter struct {
	mu       sync.Mutex
	tat      time.Time     // Theoretical Arrival Time
	emission time.Duration // Time between requests (1/rate)
	limit    time.Duration // Burst tolerance (emission * burst)
	clock    clock
}

// NewGCRALimiter creates a new GCRA limiter.
// rate is requests per second, burst is how many requests can be made instantly.
func NewGCRALimiter(rate float64, burst uint32) *GCRALimiter {
	return NewGCRALimiterWithClock(rate, burst, realClock{})
}

// NewGCRALimiterWithClock creates a new GCRA limiter with a custom clock.
func NewGCRALimiterWithClock(rate float64, burst uint32, clock clock) *GCRALimiter {
	if rate <= 0 {
		rate = 1
	}

	if burst == 0 {
		burst = 1
	}

	emission := time.Duration(float64(time.Second) / rate)
	limit := emission * time.Duration(burst)

	return &GCRALimiter{
		tat:      time.Time{}, // Zero time - allows first burst
		emission: emission,
		limit:    limit,
		clock:    clock,
	}
}

// Allow reports whether a request is allowed.
// Returns true if the request fits within the rate limit, false otherwise.
func (l *GCRALimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()

	// Calculate new TAT: max(now, old_tat) + emission
	newTAT := l.tat
	if now.After(newTAT) {
		newTAT = now
	}

	newTAT = newTAT.Add(l.emission)

	// Allow if newTAT - limit <= now
	// This means we haven't exhausted our burst credit
	allowAt := newTAT.Add(-l.limit)
	if allowAt.After(now) {
		return false
	}

	l.tat = newTAT

	return true
}
