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

// ParseRange parses a range from the string s. Valid formats are `n` and `n-m`.
func ParseRange(s string) (r Range, err error) {
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

// Ranges sends all range values to the channel ch, and the number of items to
// the channel count. Sending stops and ch and count are closed when an error
// occurs or the context is cancelled. When format is the empty string, "%d% is
// used.
func Ranges(ctx context.Context, ranges []Range, format string, ch chan<- string, count chan<- int) error {
	if format == "" {
		format = "%d"
	}

	var fullcount int
	for _, r := range ranges {
		fullcount += r.Count()
	}

	count <- fullcount

	defer close(ch)

	for _, r := range ranges {
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
