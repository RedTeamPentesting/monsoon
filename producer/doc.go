// Package produce contains several methods to generate strings.
package producer

import "context"

// Source produces a sequence of values.
type Source interface {
	// Yield sends all values to ch, and the number of items to the channel
	// count. Sending stops and ch and count are closed when an error occurs or
	// the context is cancelled. The channel count should be buffered with at a
	// size of at least one, so sending the count does not block.
	Yield(ctx context.Context, ch chan<- string, count chan<- int) error
}
