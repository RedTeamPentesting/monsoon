package producer

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Multiplexer takes several sources of values and returns the cross product.
type Multiplexer struct {
	Names      []string
	Sources    []Source
	ShowValues []bool
}

type sourceCount struct {
	sourceIndex int
	count       int
}

// AddSource adds a source with the given name.
func (m *Multiplexer) AddSource(name string, src Source, showValue bool) {
	m.Names = append(m.Names, name)
	m.Sources = append(m.Sources, src)
	m.ShowValues = append(m.ShowValues, showValue)
}

// Run runs the multiplexer until ctx is cancelled. Both ch and count will be
// closed when this function returns.
func (m *Multiplexer) Run(ctx context.Context, ch chan<- []string, count chan<- int) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer close(ch)
	defer close(count)

	var eg errgroup.Group

	// collect the counts for all sources separately, compute the global count
	// and send it on
	sourceCountChan := make(chan sourceCount, len(m.Sources))
	eg.Go(func() error {
		globalCount := 1
		countCollected := 0

		for count := range sourceCountChan {
			countCollected++
			globalCount *= count.count

			if countCollected == len(m.Sources) {
				break
			}
		}

		select {
		case count <- globalCount:
		case <-ctx.Done():
			return nil
		}

		return nil
	})

	eg.Go(func() error {
		defer close(sourceCountChan)

		return run(ctx, ch, sourceCountChan, m.Sources, nil, false)
	})

	return eg.Wait()
}

func run(ctx context.Context, resultChan chan<- []string, sourceCountChan chan<- sourceCount, sources []Source, partResult []string, countKnownSubtree bool) error {
	eg, ctx := errgroup.WithContext(ctx)

	src := sources[0]
	sources = sources[1:]

	ch := make(chan string)
	countChan := make(chan int, 1)

	eg.Go(func() error {
		return src.Yield(ctx, ch, countChan)
	})

	eg.Go(func() error {
		var c int
		select {
		case c = <-countChan:
		case <-ctx.Done():
			return nil
		}

		if !countKnownSubtree {
			sc := sourceCount{
				sourceIndex: len(partResult),
				count:       c,
			}

			select {
			case sourceCountChan <- sc:
			case <-ctx.Done():
				return nil
			}
		}

		return nil
	})

	eg.Go(func() error {
		countKnown := countKnownSubtree

		partResult := append(partResult, "")

		for {
			var (
				v  string
				ok bool
			)

			select {
			case <-ctx.Done():
				return nil
			case v, ok = <-ch:
				if !ok {
					return nil
				}
			}

			values := make([]string, len(partResult))
			copy(values, partResult)

			index := len(values) - 1

			values[index] = v

			if len(sources) > 0 {
				err := run(ctx, resultChan, sourceCountChan, sources, values, countKnown)
				if err != nil {
					return err
				}

				// make sure we only send one count result along
				countKnown = true

				continue
			}

			select {
			case resultChan <- values:
			case <-ctx.Done():
				return nil
			}
		}
	})

	return eg.Wait()
}
