package fuzz

import (
	"time"

	"github.com/juju/ratelimit"
	tomb "gopkg.in/tomb.v2"
)

// Limiter limits the number of input strings going through the channel.
type Limiter struct {
	*ratelimit.Bucket
}

// NewLimiter initializes a limiter with the given refill time and capacity.
func NewLimiter(fillInterval time.Duration, requestsPerInterval, capacity int) *Limiter {
	return &Limiter{
		Bucket: ratelimit.NewBucket(fillInterval/time.Duration(requestsPerInterval), int64(capacity)),
	}
}

// Start runs the Limiter.
func (l *Limiter) Start(t *tomb.Tomb, inputCh <-chan string, outputCh chan<- string) {
	t.Go(func() error {
		defer close(outputCh)
		for s := range inputCh {
			timeout := l.Bucket.Take(1)
			select {
			case <-time.After(timeout):
			case <-t.Dying():
				return nil
			}

			select {
			case outputCh <- s:
			case <-t.Dying():
				return nil
			}
		}
		return nil
	})
}
