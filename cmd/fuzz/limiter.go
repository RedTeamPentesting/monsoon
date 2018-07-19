package fuzz

import (
	"context"
	"time"

	"github.com/juju/ratelimit"
)

// Limiter limits the number of input strings going through the channel.
type Limiter struct {
	*ratelimit.Bucket
}

// NewLimiter initializes a limiter with the given refill time and capacity.
func NewLimiter(fillInterval time.Duration, capacity int64) *Limiter {
	return &Limiter{
		Bucket: ratelimit.NewBucket(fillInterval, capacity),
	}
}

// Start runs the Limiter in a separate goroutine. It terminates when either
// the input channel is closed or the context is cancelled.
func (l *Limiter) Start(ctx context.Context, in <-chan string) <-chan string {
	out := make(chan string)

	go func() {
		defer close(out)
		for s := range in {
			timeout := l.Bucket.Take(1)
			select {
			case <-time.After(timeout):
			case <-ctx.Done():
				return
			}

			select {
			case out <- s:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}
