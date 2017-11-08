package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// RangeProducer returns a range of integer values with a given format.
type RangeProducer struct {
	First, Last int
	Format      string
}

// Start runs a goroutine which will send all values [first, last] to the channel.
func (p *RangeProducer) Start(ctx context.Context, wg *sync.WaitGroup, ch chan<- string, count chan<- int) error {
	if p.First > p.Last {
		return errors.New("last value is smaller than first value")
	}

	if p.Format == "" {
		p.Format = "%d"
	}

	count <- p.Last - p.First + 1

	wg.Add(1)
	go p.run(ctx, wg, ch)
	return nil
}

// run sends values to the channel.
func (p *RangeProducer) run(ctx context.Context, wg *sync.WaitGroup, ch chan<- string) {
	defer wg.Done()

	for i := p.First; i <= p.Last; i++ {
		v := fmt.Sprintf(p.Format, i)
		select {
		case ch <- v:
		case <-ctx.Done():
			return
		}
	}
}
