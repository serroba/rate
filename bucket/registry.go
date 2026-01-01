package bucket

import (
	"sync"
)

type (
	Identifier string
	Registry   struct {
		mu       sync.Mutex
		factory  LimiterFactory
		limiters map[Identifier]Limiter
	}
)

type Limiter interface {
	Allow() bool
}

type LimiterFactory func(key Identifier) Limiter

func NewRegistry(factory LimiterFactory, keys ...Identifier) (*Registry, error) {
	limiters := make(map[Identifier]Limiter)

	for _, key := range keys {
		limiters[key] = factory(key)
	}

	return &Registry{
		limiters: limiters,
		factory:  factory,
	}, nil
}

func (r *Registry) Allow(key Identifier) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	lim, ok := r.limiters[key]
	if !ok {
		lim = r.factory(key)
		r.limiters[key] = lim
	}

	return lim.Allow()
}
