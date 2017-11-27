package main

import (
	"context"

	tomb "gopkg.in/tomb.v2"
)

// ValueFilter selects/rejects items received from a producer.
type ValueFilter interface {
	Filter(t *tomb.Tomb, inValue <-chan string, inCount <-chan int, outValue chan<- string, outCount chan<- int) error
}

// RunValueFilter starts a value filter in a Goroutine which filters values
// (and count) received in the channels and sends them on to the returned
// output channels.
func RunValueFilter(ctx context.Context, f ValueFilter, val chan string, count chan int) (chan string, chan int, error) {
	outVal := make(chan string)
	outCount := make(chan int, 1)

	t, _ := tomb.WithContext(ctx)

	err := f.Filter(t, val, count, outVal, outCount)
	if err != nil {
		return nil, nil, err
	}

	return outVal, outCount, nil
}

// ValueFilterSkip skips the first n values sent over the channel.
type ValueFilterSkip struct {
	Skip int
}

// Filter filters values sent over ch.
func (f *ValueFilterSkip) Filter(t *tomb.Tomb, inValue <-chan string, inCount <-chan int, outValue chan<- string, outCount chan<- int) error {
	t.Go(func() error {
		c := <-inCount

		// calculate the correct total count
		if c < f.Skip {
			c = 0
		} else {
			c -= f.Skip
		}

		outCount <- c
		return nil
	})

	t.Go(func() error {
		defer close(outValue)
		var cur int
		for {
			var v string
			var ok bool
			select {
			case <-t.Dying():
				return nil
			case v, ok = <-inValue:
				// when the input channel is closed we're done
				if !ok {
					return nil
				}
			}

			if cur < f.Skip {
				cur++
				// drop value, receive next
				continue
			}

			select {
			case <-t.Dying():
				return nil
			case outValue <- v:
			}
		}
	})

	return nil
}

// ValueFilterLimit passes through at most Max values.
type ValueFilterLimit struct {
	Max int
}

// Filter filters values sent over ch.
func (f *ValueFilterLimit) Filter(t *tomb.Tomb, inValue <-chan string, inCount <-chan int, outValue chan<- string, outCount chan<- int) error {
	t.Go(func() error {
		c := <-inCount

		// calculate the correct total count
		if c > f.Max {
			c = f.Max
		}

		outCount <- c
		return nil
	})

	t.Go(func() error {
		defer close(outValue)
		var cur int
		for {
			var v string
			var ok bool
			select {
			case <-t.Dying():
				return nil
			case v, ok = <-inValue:
				// when the input channel is closed we're done
				if !ok {
					return nil
				}
			}

			if cur >= f.Max {
				// drop value, receive next
				continue
			}
			cur++

			select {
			case <-t.Dying():
				return nil
			case outValue <- v:
			}
		}
	})

	return nil
}
