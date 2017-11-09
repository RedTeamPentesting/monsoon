package main

import (
	"errors"
	"fmt"

	tomb "gopkg.in/tomb.v2"
)

// RangeProducer returns a range of integer values with a given format.
type RangeProducer struct {
	First, Last int
	Format      string

	ch chan<- string
	t  *tomb.Tomb
}

// Start runs a goroutine which will send all values [first, last] to the channel.
func (p *RangeProducer) Start(t *tomb.Tomb, ch chan<- string, count chan<- int) error {
	if p.First > p.Last {
		return errors.New("last value is smaller than first value")
	}

	if p.Format == "" {
		p.Format = "%d"
	}

	p.ch = ch
	p.t = t
	count <- p.Last - p.First + 1

	t.Go(p.run)
	return nil
}

// run sends values to the channel.
func (p *RangeProducer) run() error {
	for i := p.First; i <= p.Last; i++ {
		v := fmt.Sprintf(p.Format, i)
		select {
		case p.ch <- v:
		case <-p.t.Dying():
			return nil
		}
	}

	return nil
}
