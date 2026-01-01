package bucket_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/serroba/rate/bucket"
	"github.com/stretchr/testify/assert"
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

func TestWithConcurrency(t *testing.T) {
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
