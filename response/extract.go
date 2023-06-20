package response

import (
	"context"
	"regexp"
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
// a separate goroutine, which terminates when the input channel is closed or
// the context is cancelled.
//
// The values that were used to produce the request are passed in the environment
// variable $MONSOON_VALUE (for the first one) and $MONSOON_VALUE1 to $MONSOON_VALUEN
// if several values were used.
func (e *Extracter) Run(ctx context.Context, in <-chan Response) <-chan Response {
	ch := make(chan Response)

	go func() {
		defer close(ch)
		for {
			var res Response

			select {
			case res = <-ch:
			case <-ctx.Done():
				return
			}

			if res.Hide || res.Error != nil {
				// forward response to next in chain
				ch <- res
				continue
			}

			err := res.ExtractBodyCommand(ctx, e.Commands)
			if err != nil && e.Error != nil {
				e.Error = err
			}

			res.ExtractBody(e.Pattern)

			// forward response to next in chain
			select {
			case ch <- res:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}
