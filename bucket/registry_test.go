package bucket_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/serroba/rate/bucket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var strategies = []struct {
	name    string
	factory func(capacity, rate uint32) bucket.LimiterFactory
}{
	{
		name: "token",
		factory: func(capacity, rate uint32) bucket.LimiterFactory {
			return func(bucket.Identifier) bucket.Limiter {
				return bucket.NewTokenLimiter(capacity, rate)
			}
		},
	},
	{
		name: "leaky",
		factory: func(capacity, rate uint32) bucket.LimiterFactory {
			return func(bucket.Identifier) bucket.Limiter {
				return bucket.NewLeakyLimiter(capacity, rate)
			}
		},
	},
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			reg, err := bucket.NewRegistry(s.factory(10, 2))
			require.NoError(t, err)
			require.NotNil(t, reg)
		})
	}
}

func TestNewRegistry_WithUsers(t *testing.T) {
	t.Parallel()

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			reg, err := bucket.NewRegistry(s.factory(10, 2), "alice", "bob")
			require.NoError(t, err)
			require.NotNil(t, reg)
		})
	}
}

func TestRegistry_Allow_ExistingUser(t *testing.T) {
	t.Parallel()

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			reg, err := bucket.NewRegistry(s.factory(2, 0), "alice")
			require.NoError(t, err)

			require.True(t, reg.Allow("alice"))
			require.True(t, reg.Allow("alice"))
			require.False(t, reg.Allow("alice"))
		})
	}
}

func TestRegistry_Allow_NewUser(t *testing.T) {
	t.Parallel()

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			reg, err := bucket.NewRegistry(s.factory(2, 0))
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

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			reg, err := bucket.NewRegistry(s.factory(1, 0))
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

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			reg, err := bucket.NewRegistry(s.factory(100, 0))
			require.NoError(t, err)

			var (
				allowed atomic.Int64
				wg      sync.WaitGroup
			)

			// 50 goroutines per user, 4 users = 200 goroutines
			users := []bucket.Identifier{"alice", "bob", "charlie", "diana"}
			for _, user := range users {
				for range 50 {
					wg.Add(1)

					go func(u bucket.Identifier) {
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

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			reg, err := bucket.NewRegistry(s.factory(100, 0))
			require.NoError(t, err)

			var (
				allowed atomic.Int64
				deny    atomic.Int64
				wg      sync.WaitGroup
			)

			// 50 goroutines per user, 4 users = 200 goroutines
			users := []bucket.Identifier{"alice", "bob", "charlie", "diana"}
			for _, user := range users {
				for range 110 {
					wg.Add(1)

					go func(u bucket.Identifier) {
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

			// Each user has capacity 100, only 50 requests each, so all should be allowed
			assert.Equal(t, int64(400), allowed.Load())
			assert.Equal(t, int64(40), deny.Load())
		})
	}
}

func TestRegistry_Allow_ConcurrentNewUsers(t *testing.T) {
	reg, err := bucket.NewRegistry(func(bucket.Identifier) bucket.Limiter {
		return bucket.NewTokenLimiter(5, 0)
	})
	require.NoError(t, err)

	var wg sync.WaitGroup

	// Create 100 different users concurrently
	for i := range 100 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			user := bucket.Identifier(rune('a' + id%26))
			reg.Allow(user)
		}(i)
	}

	wg.Wait()
	// If we get here without panic or race, the test passes
}
