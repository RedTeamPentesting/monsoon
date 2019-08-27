package response

import "regexp"

// Extract extracts data from interesting (non-hidden) responses. Extraction is
// done in a separate goroutine, which terminates when the input channel is
// closed.
func Extract(in <-chan Response, pattern []*regexp.Regexp, cmds [][]string) <-chan Response {
	ch := make(chan Response)

	go func() {
		defer close(ch)
		for res := range in {
			if res.Hide {
				continue
			}

			if res.Error != nil {
				continue
			}

			err := res.ExtractBody(pattern, cmds)
			if err != nil {
				res.Error = err
			}

			if res.Error == nil {
				err = res.ExtractHeader(res.HTTPResponse, pattern)
				if err != nil {
					res.Error = err
				}
			}

			// forward response to next in chain
			ch <- res
		}
	}()

	return ch
}
