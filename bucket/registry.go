package bucket

import (
	"sync"
)

type (
	Identifier string
	Registry   struct {
		mu             sync.Mutex
		limiters       map[Identifier]*TokenLimiter
		capacity, rate uint32
	}
)

func NewRegistry(capacity, rate uint32, users ...Identifier) (*Registry, error) {
	limiters := make(map[Identifier]*TokenLimiter)

	for _, user := range users {
		limiter := NewLimiter(capacity, rate)
		limiters[user] = limiter
	}

	return &Registry{
		limiters: limiters,
		capacity: capacity,
		rate:     rate,
	}, nil
}

func (r *Registry) Allow(key Identifier) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	lim, ok := r.limiters[key]
	if !ok {
		lim = NewLimiter(r.capacity, r.rate)
		r.limiters[key] = lim
	}

	return lim.Allow()
}
