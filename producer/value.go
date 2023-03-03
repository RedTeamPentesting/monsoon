package producer

import (
	"context"
)

// File produces items from a file.
type Value struct {
	value string
}

// statically ensure that *File implements Source
var _ Source = &Value{}

// NewFile creates a new producer from a reader.
func NewValue(value string) *Value {
	return &Value{value: value}
}

func (f *Value) Yield(ctx context.Context, ch chan<- string, count chan<- int) (err error) {
        defer close(ch)
	ch <- f.value
	count <- 1
	return nil
}
