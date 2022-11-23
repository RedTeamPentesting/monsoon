package producer

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

// File produces items from a file.
type File struct {
	rd io.ReadSeeker
}

// statically ensure that *File implements Source
var _ Source = &File{}

// NewFile creates a new producer from a reader.
func NewFile(rd io.ReadSeeker) *File {
	return &File{rd: rd}
}

// Yield sends all lines read from reader to ch, and the number of items to the
// channel count. Sending stops and ch and count are closed when an error occurs
// or the context is cancelled.
func (f *File) Yield(ctx context.Context, ch chan<- string, count chan<- int) (err error) {
	_, err = f.rd.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek: %w", err)
	}

	defer close(ch)

	sc := bufio.NewScanner(f.rd)
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
