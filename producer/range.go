package producer

import (
	"context"
	"fmt"
	"math"
	"math/bits"
	"regexp"
	"strconv"
	"strings"
)

var rangeRegex = regexp.MustCompile(`^(-?\d+(?:[eE]\d+)?)(?:-(-?\d+(?:[eE]\d+)?))?$`)

// Range defines a range of values which should be tested.
type Range struct {
	First, Last int
}

// NewRange parses a range from the string s. Valid formats are `n` and `n-m`.
func NewRange(s string) (r Range, err error) {
	matches := rangeRegex.FindStringSubmatch(s)
	switch {
	case len(matches) == 3 && matches[2] == "":
		value, err := parseInt(matches[1])
		if err != nil {
			return Range{}, fmt.Errorf("parse single value: %w", err)
		}

		return Range{First: value, Last: value}, nil
	case len(matches) == 3:
		first, err := parseInt(matches[1])
		if err != nil {
			return Range{}, fmt.Errorf("parse range start value %q: %w", matches[1], err)
		}

		last, err := parseInt(matches[2])
		if err != nil {
			return Range{}, fmt.Errorf("parse range end value: %q: %w", matches[2], err)
		}

		return Range{First: first, Last: last}, nil
	default:
		return Range{}, fmt.Errorf("invalid range expression: %s", s)

	}
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

func parseInt(s string) (int, error) {
	if !strings.HasPrefix(s, "-") && strings.Contains(s, "-") {
		return 0, fmt.Errorf("invalid integer")
	}

	value, initialErr := strconv.Atoi(s)
	if initialErr == nil {
		return value, nil
	}

	var exp int

	n, err := fmt.Sscanf(strings.TrimSpace(strings.ToLower(s))+"\n", "%de%d\n", &value, &exp)
	if err != nil || n != 2 {
		return 0, fmt.Errorf("invalid integer")
	}

	maxExp := math.Log10(math.MaxInt)
	if exp > int(maxExp) {
		return 0, fmt.Errorf("exponent %d is larger than the maximum of %d", exp, int(maxExp))
	}

	magnitude := int64(math.Pow10(exp))

	negative := false
	if value < 0 {
		value = -value
		negative = true
	}

	hi, lo := bits.Mul64(uint64(value), uint64(magnitude))
	if hi != 0 || lo > math.MaxInt {
		return 0, fmt.Errorf("%s is too large", s)
	}

	if negative {
		return -int(lo), nil
	}

	return int(lo), nil
}
