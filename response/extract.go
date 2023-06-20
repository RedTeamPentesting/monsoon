package response

import (
	"context"
	"regexp"

	"golang.org/x/sync/errgroup"
)

// Extracter collects data from interesting (non-hidden) responses.
type Extracter struct {
	Pattern  []*regexp.Regexp
	Commands [][]string
	Error    error
}

// Run extracts data from the body of a response by running external commands
// and feeding them the response body. Commands used to extract data are only
// run for non-hidden responses, since this is expensive. Extraction is done in
// several goroutines, which terminate when the input channel is closed or
// the context is cancelled.
//
// The values that were used to produce the request are passed in the environment
// variable $MONSOON_VALUE (for the first one) and $MONSOON_VALUE1 to $MONSOON_VALUEN
// if several values were used.
func (e *Extracter) Run(ctx context.Context, numWorkers int, in <-chan Response) <-chan Response {
	ch := make(chan Response)

	go func() {
		defer close(ch)

		eg, ctx := errgroup.WithContext(ctx)

		for i := 0; i < numWorkers; i++ {
			eg.Go(func() error {
				return e.handleResponses(ctx, in, ch)
			})
		}

		_ = eg.Wait()
	}()

	return ch
}

func (e Extracter) handleResponses(ctx context.Context, in <-chan Response, out chan<- Response) error {
	for {
		var res Response

		select {
		case res = <-in:
		case <-ctx.Done():
			return nil
		}

		if res.Hide || res.Error != nil {
			// forward response to next in chain
			out <- res
			continue
		}

		err := res.ExtractBodyCommand(ctx, e.Commands)
		if err != nil && e.Error != nil {
			e.Error = err
		}

		res.ExtractBody(e.Pattern)

		// forward response to next in chain
		select {
		case out <- res:
		case <-ctx.Done():
			return nil
		}
	}
}
