package producer

import (
	"context"
	"errors"
	"fmt"
)

// Range sends all values [first, last] to the channel ch, and the number of
// items to the channel count. Sending stops and ch and count are closed when
// an error occurs or the context is cancelled. When format is the empty
// string, "%d% is used.
func Range(ctx context.Context, first, last int, format string, ch chan<- string, count chan<- int) error {
	if first > last {
		return errors.New("last value is smaller than first value")
	}

	if format == "" {
		format = "%d"
	}

	count <- last - first + 1

	defer close(ch)
	for i := first; i <= last; i++ {
		v := fmt.Sprintf(format, i)
		select {
		case ch <- v:
		case <-ctx.Done():
			return nil
		}
	}

	return nil
}
