package token_test

import (
	"testing"
	"time"

	"rate/token"
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

func TestLimiter_Allow(t *testing.T) {
	type fields struct {
		capacity float64
		rate     float64
	}
	clock := &testClock{now: time.Now()}
	tests := []struct {
		name             string
		fields           fields
		previousAttempts int
		advanceBy        time.Duration
		want             bool
	}{
		{name: "Test basic", fields: fields{capacity: 0, rate: 1}, want: false},
		{name: "Test basic", fields: fields{capacity: 1, rate: 1}, want: true},
		{name: "Test After 1 attempt", fields: fields{capacity: 1, rate: 1}, previousAttempts: 1, want: false},
		{name: "Test after many attempts", fields: fields{capacity: 5, rate: 2}, previousAttempts: 4, want: true},
		{name: "Test after many attempts and 2 sec", fields: fields{capacity: 5, rate: 2}, previousAttempts: 7, advanceBy: 2 * time.Second, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lim := token.NewLimiterWithClock(tt.fields.capacity, tt.fields.rate, clock)
			for i := 0; i < tt.previousAttempts; i++ {
				lim.Allow()
			}
			clock.advance(tt.advanceBy)
			if got := lim.Allow(); got != tt.want {
				t.Errorf("Allow() = %v, want %v", got, tt.want)
			}
		})
	}
}
