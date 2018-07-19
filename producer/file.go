package producer

import (
	"bufio"
	"context"
	"io"
)

// Reader sends all lines read from reader channel ch, and the number of
// items to the channel count. Sending stops and ch and count are closed when
// an error occurs or the context is cancelled. The reader is closed when this
// function returns.
func Reader(ctx context.Context, rd io.ReadCloser, ch chan<- string, count chan<- int) (err error) {
	defer close(ch)
	defer func() {
		// ignore error
		_ = rd.Close()
	}()

	sc := bufio.NewScanner(rd)
	num := 0
	for sc.Scan() {
		if sc.Err() != nil {
			return sc.Err()
		}

		num++

		select {
		case ch <- sc.Text():
		case <-ctx.Done():
			return nil
		}
	}
	count <- num
	return nil
}
