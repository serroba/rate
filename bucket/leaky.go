package bucket

import (
	"sync"
	"time"
)

type LeakyLimiter struct {
	mu sync.Mutex

	capacity, level, rate float64
	lastUpdatedAt         time.Time
}

func NewLeakyLimiter(capacity, rate uint32) *LeakyLimiter {
	return &LeakyLimiter{
		capacity:      float64(capacity),
		rate:          float64(rate),
		lastUpdatedAt: time.Now(),
	}
}

func (lim *LeakyLimiter) update() {
	t := time.Now()
	if t.Before(lim.lastUpdatedAt) {
		return
	}

	lim.level = max(0, lim.level-t.Sub(lim.lastUpdatedAt).Seconds()*lim.rate)
	lim.lastUpdatedAt = t
}

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
