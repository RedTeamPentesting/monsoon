package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
)

// FileProducer returns each line read from a file.
type FileProducer struct {
	Filename string
	f        *os.File
}

// Start runs a goroutine which will send all values [first, last] to the channel.
func (p *FileProducer) Start(ctx context.Context, wg *sync.WaitGroup, ch chan<- string, count chan<- int) (err error) {
	p.f, err = os.Open(p.Filename)
	if err != nil {
		return err
	}

	wg.Add(1)
	go p.run(ctx, wg, ch, count)
	return nil
}

// run sends values to the channel.
func (p *FileProducer) run(ctx context.Context, wg *sync.WaitGroup, ch chan<- string, count chan<- int) {
	defer wg.Done()

	sc := bufio.NewScanner(p.f)
	num := 0
	for sc.Scan() {
		if sc.Err() != nil {
			fmt.Fprintf(os.Stderr, "error reading %v: %v\n", p.Filename, sc.Err())
			return
		}

		num++

		select {
		case ch <- sc.Text():
		case <-ctx.Done():
			return
		}
	}
	count <- num
}
