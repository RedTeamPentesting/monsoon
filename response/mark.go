package response

// Mark runs all responses through filters and sets the Hide attribute if a
// filter matches. Filtering is done in a separate goroutine, which terminates
// when the input channel is closed.
func Mark(in <-chan Response, filters []Filter) <-chan Response {
	ch := make(chan Response)

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
