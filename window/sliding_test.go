package window_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/serroba/rate/window"
	"github.com/stretchr/testify/require"
)

func TestNewSlidingLimiter_DefaultWindow(t *testing.T) {
	t.Parallel()

	// Should not panic with zero window
	lim := window.NewSlidingLimiter(10, 0)
	require.NotNil(t, lim)
	require.True(t, lim.Allow())
}

func TestSlidingLimiter_Allow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		limit            uint32
		window           time.Duration
		previousAttempts int
		want             bool
	}{
		{
			name:   "zero limit rejects immediately",
			limit:  0,
			window: time.Hour,
			want:   false,
		},
		{
			name:   "allows first request",
			limit:  1,
			window: time.Hour,
			want:   true,
		},
		{
			name:             "rejects after limit reached",
			limit:            1,
			window:           time.Hour,
			previousAttempts: 1,
			want:             false,
		},
		{
			name:             "allows up to limit",
			limit:            5,
			window:           time.Hour,
			previousAttempts: 4,
			want:             true,
		},
		{
			name:             "rejects at limit",
			limit:            5,
			window:           time.Hour,
			previousAttempts: 5,
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lim := window.NewSlidingLimiter(tt.limit, tt.window)

			for range tt.previousAttempts {
				lim.Allow()
			}

			got := lim.Allow()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSlidingLimiter_Allow_Concurrent(t *testing.T) {
	t.Parallel()

	lim := window.NewSlidingLimiter(100, time.Hour)

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

	require.Equal(t, int64(100), allowed.Load())
}

func TestSlidingLimiter_Allow_ConcurrentHammer(t *testing.T) {
	t.Parallel()

	lim := window.NewSlidingLimiter(50, time.Hour)

	var wg sync.WaitGroup

	// Hammer from multiple goroutines
	for range 100 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 100 {
				lim.Allow()
			}
		}()
	}

	wg.Wait()
	// Pass if no race detected
}

func TestSlidingLimiter_Allow_WindowExpiry(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)}
	lim := window.NewSlidingLimiterWithClock(2, time.Minute, clock)

	// Use up the limit
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())

	// Advance past the window
	clock.advance(time.Minute + time.Second)

	// Should be allowed again
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())
}

func TestSlidingLimiter_Allow_PartialExpiry(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)}
	lim := window.NewSlidingLimiterWithClock(2, time.Minute, clock)

	// First request at t=0
	require.True(t, lim.Allow())

	// Advance 30 seconds
	clock.advance(30 * time.Second)

	// Second request at t=30s
	require.True(t, lim.Allow())

	// Third should be rejected (both requests still in window)
	require.False(t, lim.Allow())

	// Advance another 35 seconds (t=65s) - first request expires, second still valid
	clock.advance(35 * time.Second)

	// Now one slot available (first expired, second still in window)
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())
}

func TestSlidingLimiter_Allow_SameWindowNoExpiry(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)}
	lim := window.NewSlidingLimiterWithClock(2, time.Minute, clock)

	// Use up the limit
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())

	// Advance but stay within window
	clock.advance(30 * time.Second)

	// Still rejected
	require.False(t, lim.Allow())
}
