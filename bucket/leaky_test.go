package bucket_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/serroba/rate/bucket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeakyLimiter_Allow(t *testing.T) {
	type args struct {
		capacity uint32
		rate     uint32
	}

	tests := []struct {
		name             string
		args             args
		previousAttempts int
		want             bool
	}{
		{name: "Test With No Capacity", args: args{capacity: 0, rate: 0}, want: false},
		{name: "Test With Capacity 1", args: args{capacity: 1, rate: 0}, want: true},
		{
			name:             "Test With Capacity 1 with previous attempt",
			args:             args{capacity: 1, rate: 0},
			previousAttempts: 1,
			want:             false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lim := bucket.NewLeakyLimiter(tt.args.capacity, tt.args.rate)
			for range tt.previousAttempts {
				lim.Allow()
			}

			assert.Equalf(t, tt.want, lim.Allow(), "Allow()")
		})
	}
}

func TestLeakyLimiter_Allow_Concurrent(t *testing.T) {
	var (
		allow atomic.Int64
		deny  atomic.Int64
		wg    sync.WaitGroup
	)

	lim := bucket.NewLeakyLimiter(10, 5)

	for range 15 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if lim.Allow() {
				allow.Add(1)
			} else {
				deny.Add(1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(10), allow.Load())
	assert.Equal(t, int64(5), deny.Load())
}

func TestLeakyLimiter_Allow_ClockGoesBackwards(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Now()}
	lim := bucket.NewLeakyLimiterWithClock(1, 1, clock)

	// Fill the bucket
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())

	// Move clock backwards - should not drain
	clock.now = clock.now.Add(-1 * time.Second)

	require.False(t, lim.Allow())
}

func TestLeakyLimiter_Allow_Drains(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Now()}
	lim := bucket.NewLeakyLimiterWithClock(2, 2, clock) // drains 2 per second

	// Fill the bucket
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())

	// Advance 1 second - should drain 2, bucket now empty
	clock.advance(1 * time.Second)

	// Can fill again
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())
}

func TestLeakyLimiter_Allow_PartialDrain(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Now()}
	lim := bucket.NewLeakyLimiterWithClock(2, 2, clock) // drains 2 per second

	// Fill the bucket
	require.True(t, lim.Allow())
	require.True(t, lim.Allow())
	require.False(t, lim.Allow())

	// Advance 0.5 seconds - should drain 1, leaving room for 1
	clock.advance(500 * time.Millisecond)

	require.True(t, lim.Allow())
	require.False(t, lim.Allow())
}
