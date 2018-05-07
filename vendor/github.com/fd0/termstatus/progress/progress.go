package progress

import (
	"fmt"
	"io"
	"time"
)

// Terminal allows writing messages which stay on the screen and scroll, and
// displaying a set of status lines that are updated whenever new information
// is avaliable.
type Terminal interface {
	Printf(string, ...interface{})
	SetStatus([]string)
}

type progressReader struct {
	io.Reader
	term  Terminal
	last  time.Time
	bytes int
}

func (rd *progressReader) Read(p []byte) (int, error) {
	n, err := rd.Reader.Read(p)

	rd.bytes += n
	var bps float32
	sec := float32(time.Since(rd.last)) / float32(time.Second)
	if sec > 0 {
		bps = float32(n) / sec
	}
	rd.last = time.Now()

	status := fmt.Sprintf("read %d bytes (%.0f b/s)\n", rd.bytes, bps)
	rd.term.SetStatus([]string{status})
	return n, err
}

// Reader returns a wrapped reader which reports progress statios via term.
func Reader(rd io.Reader, term Terminal) io.Reader {
	return &progressReader{Reader: rd, term: term}
}
