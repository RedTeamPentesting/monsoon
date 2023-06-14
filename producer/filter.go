package producer

import "context"

// Filter selects/rejects items received from a source.
type Filter interface {
	// Count corrects the number of total items to test
	Count(ctx context.Context, in <-chan int) <-chan int

	// Select filters the items
	Select(ctx context.Context, in <-chan []string) <-chan []string
}

// FilterSkip skips the first n values sent over the channel.
type FilterSkip struct {
	Skip int
}

// ensure Filter Skip implements Filter
var _ Filter = &FilterSkip{}

// Count filters the number of values.
func (f *FilterSkip) Count(ctx context.Context, in <-chan int) <-chan int {
	out := make(chan int, 1)

	go func() {
		defer close(out)
		var total int
		select {
		case total = <-in:
		case <-ctx.Done():
		}

		// calculate the correct total count
		if total < f.Skip {
			total = 0
		} else {
			total -= f.Skip
		}

		select {
		case out <- total:
		case <-ctx.Done():
		}
	}()

	return out
}

// Select filters values sent over ch.
func (f *FilterSkip) Select(ctx context.Context, in <-chan []string) <-chan []string {
	out := make(chan []string)

	go func() {
		defer close(out)
		var cur int
		for {
			var v []string
			var ok bool
			select {
			case <-ctx.Done():
				return
			case v, ok = <-in:
				// when the input channel is closed we're done
				if !ok {
					return
				}
			}

			if cur < f.Skip {
				cur++
				// drop value, receive next
				continue
			}

			select {
			case <-ctx.Done():
				return
			case out <- v:
			}
		}
	}()

	return out
}

// FilterLimit passes through at most Max values.
type FilterLimit struct {
	Max            int
	CancelProducer func()
}

// ensure FilterLimit implements Filter.
var _ Filter = &FilterLimit{}

// Count filters the number of values.
func (f *FilterLimit) Count(ctx context.Context, in <-chan int) <-chan int {
	out := make(chan int, 1)

	go func() {
		defer close(out)
		var total int
		select {
		case total = <-in:
		case <-ctx.Done():
		}

		// calculate the correct total count
		if total > f.Max {
			total = f.Max
		}

		select {
		case out <- total:
		case <-ctx.Done():
		}
	}()

	return out
}

// Select filters values sent over ch.
func (f *FilterLimit) Select(ctx context.Context, in <-chan []string) <-chan []string {
	out := make(chan []string)

	go func() {
		defer close(out)
		defer f.CancelProducer()

		var cur int
		for {
			var v []string
			var ok bool
			select {
			case <-ctx.Done():
				return
			case v, ok = <-in:
				// when the input channel is closed we're done
				if !ok {
					return
				}
			}

			if cur >= f.Max {
				// cancel producer, drop value, receive next
				f.CancelProducer()
				continue
			}
			cur++

			select {
			case <-ctx.Done():
				return
			case out <- v:
			}
		}
	}()

	return out
}
