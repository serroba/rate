package bucket_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/serroba/rate/bucket"
	"github.com/stretchr/testify/require"
)

func TestGCRALimiter_Allow_Burst(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Now()}
	// 10 requests/second, burst of 3
	lim := bucket.NewGCRALimiterWithClock(10, 3, clock)

	// Should allow burst of 3 instantly
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())

	// 4th should be rejected (burst exhausted)
	require.False(t, lim.Allow())
}

func TestGCRALimiter_Allow_RateLimit(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Now()}
	// 2 requests/second, burst of 1
	lim := bucket.NewGCRALimiterWithClock(2, 1, clock)

	// First request allowed
	require.True(t, lim.Allow())

	// Second immediately rejected (no burst)
	require.False(t, lim.Allow())

	// Advance 500ms (half the interval)
	clock.advance(500 * time.Millisecond)
	require.True(t, lim.Allow())

	// Immediately rejected again
	require.False(t, lim.Allow())
}

func TestGCRALimiter_Allow_RefillsOverTime(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Now()}
	// 10 requests/second, burst of 3
	lim := bucket.NewGCRALimiterWithClock(10, 3, clock)

	// Exhaust burst
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())

	// Advance 100ms = 1 request worth
	clock.advance(100 * time.Millisecond)
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())

	// Advance 200ms = 2 more requests worth
	clock.advance(200 * time.Millisecond)
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())
}

func TestGCRALimiter_Allow_IdleAccumulatesCredit(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Now()}
	// 10 requests/second, burst of 5
	lim := bucket.NewGCRALimiterWithClock(10, 5, clock)

	// Use 2 of 5 burst
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())

	// Go idle for 1 second (10 requests worth, but capped at burst=5)
	clock.advance(1 * time.Second)

	// Should have full burst again
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())
}

func TestGCRALimiter_Allow_Concurrent(t *testing.T) {
	t.Parallel()

	// 1000 requests/second, burst of 100
	lim := bucket.NewGCRALimiter(1000, 100)

	var (
		allowed atomic.Int64
		wg      sync.WaitGroup
	)

	for range 200 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if lim.Allow() {
				allowed.Add(1)
			}
		}()
	}

	wg.Wait()

	// Should allow approximately burst amount (100)
	require.Equal(t, int64(100), allowed.Load())
}

func TestNewGCRALimiter_DefaultValues(t *testing.T) {
	t.Parallel()

	// Zero/negative values should use defaults
	lim := bucket.NewGCRALimiter(0, 0)
	require.NotNil(t, lim)
	require.True(t, lim.Allow())
}
