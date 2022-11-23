package producer

import (
	"context"
	"fmt"
	"strconv"
)

// Range defines a range of values which should be tested.
type Range struct {
	First, Last int
}

// NewRange parses a range from the string s. Valid formats are `n` and `n-m`.
func NewRange(s string) (r Range, err error) {
	// test if it's a number only
	n, err := strconv.Atoi(s)
	if err == nil {
		return Range{First: n, Last: n}, nil
	}

	// otherwise assume it's a range
	_, err = fmt.Sscanf(s, "%d-%d", &r.First, &r.Last)
	if err != nil {
		return Range{}, fmt.Errorf("wrong format for range, expected: first-last, got: %q", s)
	}

	return r, nil
}

// Count returns the number of items in the range.
func (r Range) Count() int {
	if r.Last > r.First {
		return r.Last - r.First + 1
	}

	return r.First - r.Last + 1
}

// Ranges is a source which yields values from several ranges.
type Ranges struct {
	ranges []Range
	format string
}

// statically ensure that *Ranges implements Source.
var _ Source = &Ranges{}

// NewRanges initializes a new source for several ranges. If format is the empty
// string, "%d" is used.
func NewRanges(ranges []Range, format string) *Ranges {
	if format == "" {
		format = "%d"
	}

	return &Ranges{ranges: ranges, format: format}
}

// Yield sends all lines read from reader to ch, and the number of items to the
// channel count. Sending stops and ch and count are closed when an error occurs
// or the context is cancelled. The reader is closed when this function returns.
func (r *Ranges) Yield(ctx context.Context, ch chan<- string, count chan<- int) (err error) {
	var fullcount int
	for _, r := range r.ranges {
		fullcount += r.Count()
	}

	count <- fullcount

	defer close(ch)

	format := r.format

	for _, r := range r.ranges {
		increment := 1
		last := r.Last + 1

		if r.Last < r.First {
			increment = -1
			last = r.Last - 1
		}

		for i := r.First; i != last; i += increment {
			v := fmt.Sprintf(format, i)
			select {
			case ch <- v:
			case <-ctx.Done():
				return nil
			}
		}
	}

	return nil
}
