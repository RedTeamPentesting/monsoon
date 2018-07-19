package fuzz

import "context"

// Producer yields values for enumerating.
type Producer interface {
	Start(context.Context, chan<- string, chan<- int) error
}
