package window_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/serroba/rate/window"
	"github.com/stretchr/testify/require"
)

type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time {
	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func TestNewFixedLimiter_DefaultWindow(t *testing.T) {
	t.Parallel()

	// Should not panic with zero window
	lim := window.NewFixedLimiter(10, 0)
	require.NotNil(t, lim)
	require.True(t, lim.Allow())
}

func TestFixedLimiter_Allow(t *testing.T) {
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
			window: time.Second,
			want:   false,
		},
		{
			name:   "allows first request",
			limit:  1,
			window: time.Second,
			want:   true,
		},
		{
			name:             "rejects after limit reached",
			limit:            1,
			window:           time.Second,
			previousAttempts: 1,
			want:             false,
		},
		{
			name:             "allows up to limit",
			limit:            5,
			window:           time.Second,
			previousAttempts: 4,
			want:             true,
		},
		{
			name:             "rejects at limit",
			limit:            5,
			window:           time.Second,
			previousAttempts: 5,
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lim := window.NewFixedLimiter(tt.limit, tt.window)

			for range tt.previousAttempts {
				lim.Allow()
			}

			got := lim.Allow()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFixedLimiter_Allow_Concurrent(t *testing.T) {
	t.Parallel()

	lim := window.NewFixedLimiter(100, time.Minute)

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

func TestFixedLimiter_Allow_ConcurrentMultipleWindows(t *testing.T) {
	t.Parallel()

	lim := window.NewFixedLimiter(50, time.Minute)

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

func TestFixedLimiter_Allow_WindowReset(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)}
	lim := window.NewFixedLimiterWithClock(2, time.Minute, clock)

	// Use up the limit
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())

	// Advance to next window
	clock.advance(time.Minute)

	// Should be allowed again
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())
}

func TestFixedLimiter_Allow_SameWindowNoReset(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)}
	lim := window.NewFixedLimiterWithClock(2, time.Minute, clock)

	// Use up the limit
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())

	// Advance but stay in same window
	clock.advance(30 * time.Second)

	// Should still be rejected
	require.False(t, lim.Allow())
}
