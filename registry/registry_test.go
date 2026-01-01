package registry_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/serroba/rate/bucket"
	"github.com/serroba/rate/registry"
	"github.com/serroba/rate/window"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StrategyConfig defines how to build a limiter factory for a specific strategy.
type StrategyConfig interface {
	Name() string
	Build() registry.LimiterFactory
}

// TokenBucketConfig configures a token bucket rate limiter.
type TokenBucketConfig struct {
	capacity uint32
	rate     uint32
}

func (c TokenBucketConfig) Name() string { return "token bucket" }

func (c TokenBucketConfig) Build() registry.LimiterFactory {
	return func() registry.Limiter {
		return bucket.NewTokenLimiter(c.capacity, c.rate)
	}
}

// LeakyBucketConfig configures a leaky bucket rate limiter.
type LeakyBucketConfig struct {
	capacity uint32
	rate     uint32
}

func (c LeakyBucketConfig) Name() string { return "leaky bucket" }

func (c LeakyBucketConfig) Build() registry.LimiterFactory {
	return func() registry.Limiter {
		return bucket.NewLeakyLimiter(c.capacity, c.rate)
	}
}

// FixedWindowConfig configures a fixed window rate limiter.
type FixedWindowConfig struct {
	limit  uint32
	window time.Duration
}

func (c FixedWindowConfig) Name() string { return "fixed window" }

func (c FixedWindowConfig) Build() registry.LimiterFactory {
	w := c.window
	if w == 0 {
		w = time.Hour
	}

	return func() registry.Limiter {
		return window.NewFixedLimiter(c.limit, w)
	}
}

type SlidingWindowConfig struct {
	limit    uint32
	duration time.Duration
}

func (s SlidingWindowConfig) Name() string { return "sliding window" }

func (s SlidingWindowConfig) Build() registry.LimiterFactory {
	return func() registry.Limiter {
		return window.NewSlidingLimiter(s.limit, s.duration)
	}
}

// Helper to create all strategies with given parameters.
func allStrategies(capacity uint32, rate uint32, win time.Duration) []StrategyConfig {
	return []StrategyConfig{
		TokenBucketConfig{capacity: capacity, rate: rate},
		LeakyBucketConfig{capacity: capacity, rate: rate},
		FixedWindowConfig{limit: capacity, window: win},
		SlidingWindowConfig{limit: capacity, duration: win},
	}
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	strategies := allStrategies(10, 2, time.Minute)
	for _, s := range strategies {
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()

			reg, err := registry.NewRegistry(s.Build())
			require.NoError(t, err)
			require.NotNil(t, reg)
		})
	}
}

func TestNewRegistry_WithUsers(t *testing.T) {
	t.Parallel()

	strategies := allStrategies(10, 2, time.Minute)
	for _, s := range strategies {
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()

			reg, err := registry.NewRegistry(s.Build(), "alice", "bob")
			require.NoError(t, err)
			require.NotNil(t, reg)
		})
	}
}

func TestRegistry_Allow_ExistingUser(t *testing.T) {
	t.Parallel()

	strategies := allStrategies(2, 0, time.Hour)
	for _, s := range strategies {
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()

			reg, err := registry.NewRegistry(s.Build(), "alice")
			require.NoError(t, err)

			require.True(t, reg.Allow("alice"))
			require.True(t, reg.Allow("alice"))
			require.False(t, reg.Allow("alice"))
		})
	}
}

func TestRegistry_Allow_NewUser(t *testing.T) {
	t.Parallel()

	strategies := allStrategies(2, 0, time.Hour)
	for _, s := range strategies {
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()

			reg, err := registry.NewRegistry(s.Build())
			require.NoError(t, err)

			// First call for a new user should create limiter and allow
			require.True(t, reg.Allow("alice"))
			require.True(t, reg.Allow("alice"))
			require.False(t, reg.Allow("alice"))
		})
	}
}

func TestRegistry_Allow_IndependentUsers(t *testing.T) {
	t.Parallel()

	strategies := allStrategies(1, 0, time.Hour)
	for _, s := range strategies {
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()

			reg, err := registry.NewRegistry(s.Build())
			require.NoError(t, err)

			// Each user has their own bucket
			require.True(t, reg.Allow("alice"))
			require.True(t, reg.Allow("bob"))

			// Both exhausted now
			require.False(t, reg.Allow("alice"))
			require.False(t, reg.Allow("bob"))
		})
	}
}

func TestRegistry_Allow_Concurrent(t *testing.T) {
	t.Parallel()

	strategies := allStrategies(100, 0, time.Hour)
	for _, s := range strategies {
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()

			reg, err := registry.NewRegistry(s.Build())
			require.NoError(t, err)

			var (
				allowed atomic.Int64
				wg      sync.WaitGroup
			)

			// 50 goroutines per user, 4 users = 200 goroutines
			users := []registry.Identifier{"alice", "bob", "charlie", "diana"}
			for _, user := range users {
				for range 50 {
					wg.Add(1)

					go func(u registry.Identifier) {
						defer wg.Done()

						if reg.Allow(u) {
							allowed.Add(1)
						}
					}(user)
				}
			}

			wg.Wait()

			// Each user has capacity 100, only 50 requests each, so all should be allowed
			require.Equal(t, int64(200), allowed.Load())
		})
	}
}

func TestRegistry_Deny_Concurrent(t *testing.T) {
	t.Parallel()

	strategies := allStrategies(100, 0, time.Hour)
	for _, s := range strategies {
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()

			reg, err := registry.NewRegistry(s.Build())
			require.NoError(t, err)

			var (
				allowed atomic.Int64
				deny    atomic.Int64
				wg      sync.WaitGroup
			)

			users := []registry.Identifier{"alice", "bob", "charlie", "diana"}
			for _, user := range users {
				for range 110 {
					wg.Add(1)

					go func(u registry.Identifier) {
						defer wg.Done()

						if reg.Allow(u) {
							allowed.Add(1)
						} else {
							deny.Add(1)
						}
					}(user)
				}
			}

			wg.Wait()

			assert.Equal(t, int64(400), allowed.Load())
			assert.Equal(t, int64(40), deny.Load())
		})
	}
}

func TestRegistry_Allow_ConcurrentNewUsers(t *testing.T) {
	t.Parallel()

	strategies := allStrategies(5, 0, time.Hour)
	for _, s := range strategies {
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()

			reg, err := registry.NewRegistry(s.Build())
			require.NoError(t, err)

			var wg sync.WaitGroup

			// Create 100 different users concurrently
			for i := range 100 {
				wg.Add(1)

				go func(id int) {
					defer wg.Done()

					user := registry.Identifier(rune('a' + id%26))
					reg.Allow(user)
				}(i)
			}

			wg.Wait()
			// If we get here without panic or race, the test passes
		})
	}
}
