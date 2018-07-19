package fuzz

import (
	"context"
	"errors"
	"fmt"
)

// RangeProducer returns a range of integer values with a given format.
type RangeProducer struct {
	First, Last int
	Format      string

	ch chan<- string
}

// Start sends all values [first, last] to the channel ch, and the number of items to the channel count.
func (p *RangeProducer) Start(ctx context.Context, ch chan<- string, count chan<- int) error {
	if p.First > p.Last {
		return errors.New("last value is smaller than first value")
	}

	if p.Format == "" {
		p.Format = "%d"
	}

	p.ch = ch
	count <- p.Last - p.First + 1

	defer close(p.ch)
	for i := p.First; i <= p.Last; i++ {
		v := fmt.Sprintf(p.Format, i)
		select {
		case p.ch <- v:
		case <-ctx.Done():
			return nil
		}
	}

	return nil
}
