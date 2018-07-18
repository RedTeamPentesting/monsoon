package fuzz

import "github.com/happal/monsoon/response"

// FilterResponses runs all responses through filters and sets the Hide
// attribute if a filter matches. Filtering is done in a separate goroutine,
// which terminates when the input channel is closed.
func FilterResponses(in <-chan response.Response, filters []response.Filter) <-chan response.Response {
	ch := make(chan response.Response)

	go func() {
		defer close(ch)
		for res := range in {
			// run filters
			hide := false
			for _, f := range filters {
				if f.Reject(res) {
					hide = true
					break
				}
			}
			res.Hide = hide

			// forward response to next in chain
			ch <- res
		}
	}()

	return ch
}
