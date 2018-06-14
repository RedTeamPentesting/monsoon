package fuzz

import (
	"bufio"
	"fmt"
	"os"

	tomb "gopkg.in/tomb.v2"
)

// FileProducer returns each line read from a file.
type FileProducer struct {
	Filename string

	f     *os.File
	count chan<- int
	ch    chan<- string
	t     *tomb.Tomb
}

// Start runs a goroutine which will send all values [first, last] to the channel.
func (p *FileProducer) Start(t *tomb.Tomb, ch chan<- string, count chan<- int) (err error) {
	p.f, err = os.Open(p.Filename)
	if err != nil {
		return err
	}
	p.count = count
	p.ch = ch
	p.t = t

	t.Go(p.run)
	return nil
}

// run sends values to the channel.
func (p *FileProducer) run() error {
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
		case <-p.t.Dying():
			return nil
		}
	}
	p.count <- num
	return nil
}
