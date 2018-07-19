package producer

import (
	"context"
	"time"

	"github.com/juju/ratelimit"
)

// Limit limits the number of values per second to the value perSecond. A new
// goroutine is started, which terminates when in is closed or the context is
// cancelled.
func Limit(ctx context.Context, perSecond float64, in <-chan string) <-chan string {
	fillInterval := time.Duration(float64(time.Second) / float64(perSecond))
	bucket := ratelimit.NewBucket(fillInterval, 1)

	out := make(chan string)

	go func() {
		defer close(out)
		for s := range in {
			timeout := bucket.Take(1)
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
