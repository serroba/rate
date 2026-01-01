package bucket_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/serroba/rate/bucket"
	"github.com/stretchr/testify/require"
)

type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time {
	return c.now
}

func (c *testClock) advance(by time.Duration) {
	c.now = c.now.Add(by)
}

func TestLimiter_Allow_ClockGoesBackwards(t *testing.T) {
	clock := &testClock{now: time.Now()}
	lim := bucket.NewLimiterWithClock(1, 1, clock)

	// Drain the bucket
	require.True(t, lim.Allow())

	// Move clock backwards - should not refill
	clock.now = clock.now.Add(-1 * time.Second)

	require.False(t, lim.Allow())
}

func TestLimiter_Allow(t *testing.T) {
	type fields struct {
		capacity uint32
		rate     uint32
	}

	clock := &testClock{now: time.Now()}

	tests := []struct {
		name             string
		fields           fields
		previousAttempts int
		advanceBy        time.Duration
		want             bool
	}{
		{name: "Test with zero capacity", fields: fields{capacity: 0, rate: 1}, want: false},
		{name: "Test with capacity of one", fields: fields{capacity: 1, rate: 1}, want: true},
		{
			name:             "Test After 1 attempt",
			fields:           fields{capacity: 1, rate: 1},
			previousAttempts: 1,
			want:             false,
		},
		{
			name:             "Test after many attempts",
			fields:           fields{capacity: 5, rate: 2},
			previousAttempts: 4,
			want:             true,
		},
		{
			name:             "Test after many attempts and 2 sec",
			fields:           fields{capacity: 5, rate: 2},
			previousAttempts: 7,
			advanceBy:        2 * time.Second,
			want:             true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lim := bucket.NewLimiterWithClock(tt.fields.capacity, tt.fields.rate, clock)

			for range tt.previousAttempts {
				lim.Allow()
			}

			clock.advance(tt.advanceBy)

			if got := lim.Allow(); got != tt.want {
				t.Errorf("Allow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLimiter_Allow_Concurrent(t *testing.T) {
	lim := bucket.NewLimiter(100, 0)

	var (
		allowed atomic.Int64
		wg      sync.WaitGroup
	)

	// Launch 200 goroutines, but only 100 should be allowed

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

	// With capacity 100 and rate 0, exactly 100 should be allowed
	require.Equal(t, int64(100), allowed.Load(), "expected exactly 100 requests to be allowed")
}

func TestLimiter_Allow_ConcurrentWithRefill(t *testing.T) {
	clock := &testClock{now: time.Now()}
	lim := bucket.NewLimiterWithClock(10, 1000, clock)

	var (
		allowed atomic.Int64
		wg      sync.WaitGroup
	)

	// Hammer the limiter from multiple goroutines
	// Clock doesn't advance, so no refill happens - exactly 10 should be allowed
	for range 100 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 100 {
				if lim.Allow() {
					allowed.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	require.Equal(t, int64(10), allowed.Load())
}
