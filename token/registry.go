package token

import (
	"fmt"
	"sync"
)

type (
	Identifier string
	Registry   struct {
		mu             sync.Mutex
		limiters       map[Identifier]*Limiter
		capacity, rate float64
	}
)

func NewRegistry(capacity, rate float64, users ...Identifier) (*Registry, error) {
	limiters := make(map[Identifier]*Limiter)

	for _, user := range users {
		limiter, err := NewLimiter(capacity, rate)
		if err != nil {
			return nil, fmt.Errorf("fail to create a new limiter %w", err)
		}

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
		lim, _ = NewLimiter(r.capacity, r.rate)
		r.limiters[key] = lim
	}

	return lim.Allow()
}
