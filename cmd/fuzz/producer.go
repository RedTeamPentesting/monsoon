package fuzz

import (
	tomb "gopkg.in/tomb.v2"
)

// Producer yields values for enumerating.
type Producer interface {
	Start(*tomb.Tomb, chan<- string, chan<- int) error
}
