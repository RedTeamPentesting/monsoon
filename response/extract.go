package response

import "regexp"

// Extracter collects data from interesting (non-hidden) responses.
type Extracter struct {
	Pattern  []*regexp.Regexp
	Commands [][]string
	Error    func(error)
}

// Run extracts data from the body of a response by running external commands
// and feeding them the response body. Commands used to extract data are only
// run for non-hidden responses, since this is expensive. Extraction is done in
// a separate goroutine, which terminates when the input channel is closed.
func (e *Extracter) Run(in <-chan Response) <-chan Response {
	ch := make(chan Response)

	go func() {
		defer close(ch)
		for res := range in {
			if res.Hide || res.Error != nil {
				// forward response to next in chain
				ch <- res
				continue
			}

			err := res.ExtractBodyCommand(e.Commands)
			if err != nil && e.Error != nil {
				e.Error(err)
			}

			res.ExtractBody(e.Pattern)

			// forward response to next in chain
			ch <- res
		}
	}()

	return ch
}
