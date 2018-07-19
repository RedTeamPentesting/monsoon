package fuzz

import (
	"bufio"
	"context"
	"fmt"
	"os"
)

// FileProducer returns each line read from a file.
type FileProducer struct {
	Filename string

	f     *os.File
	count chan<- int
	ch    chan<- string
}

// Start sends all strings from the file to the channel ch, and the number of items to the channel count.
func (p *FileProducer) Start(ctx context.Context, ch chan<- string, count chan<- int) (err error) {
	if p.Filename == "-" {
		// use stdin
		p.f = os.Stdin
	} else {
		var err error
		p.f, err = os.Open(p.Filename)
		if err != nil {
			return err
		}
	}
	p.count = count
	p.ch = ch

	defer close(p.ch)

	sc := bufio.NewScanner(p.f)
	num := 0
	for sc.Scan() {
		if sc.Err() != nil {
			fmt.Fprintf(os.Stderr, "error reading %v: %v\n", p.Filename, sc.Err())
			return sc.Err()
		}

		num++

		select {
		case p.ch <- sc.Text():
		case <-ctx.Done():
			return nil
		}
	}
	p.count <- num
	return nil
}
