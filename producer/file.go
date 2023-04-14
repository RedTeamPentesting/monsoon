package producer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
)

// File produces items from a file.
type File struct {
	rd       io.ReadSeeker
	seekable bool
	yields   int
}

// statically ensure that *File implements Source
var _ Source = &File{}

// NewFile creates a new producer from a reader. If seekable is set to false
// (e.g. for stdin), Yield() returns an error for subsequent runs.
func NewFile(rd io.ReadSeeker, seekable bool) *File {
	return &File{rd: rd, seekable: seekable}
}

// Yield sends all lines read from reader to ch, and the number of items to the
// channel count. Sending stops and ch and count are closed when an error occurs
// or the context is cancelled.
func (f *File) Yield(ctx context.Context, ch chan<- string, count chan<- int) (err error) {
	if f.yields > 0 {
		// reset the pointer to the start if possible
		if !f.seekable {
			return errors.New("file source is not seekable, can only be used as first source")
		}

		_, err = f.rd.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("seek: %w", err)
		}
	}

	f.yields++

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
