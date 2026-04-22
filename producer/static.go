package producer

import (
	"context"
	"fmt"
)

// Static handles replacements that just have a single value. They are
// generated on demand, so the value can be time-dependent (like an
// authentication token).
type Static struct {
	Names    []string
	Generate []func(context.Context) (string, error)
}

// AddSource adds a static value.
func (s *Static) AddSource(name string, f func(context.Context) (string, error)) {
	s.Generate = append(s.Generate, f)
	s.Names = append(s.Names, name)
}

// Yield returns the current set of replacement values.
func (s *Static) Yield(ctx context.Context) (values []string, err error) {
	for i, name := range s.Names {
		v, err := s.Generate[i](ctx)
		if err != nil {
			return nil, fmt.Errorf("generate value for static replacement %q: %w", name, err)
		}

		values = append(values, v)
	}

	return values, nil
}
