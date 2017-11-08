package main

// SimpleFilter hides responses based on HTTP response code.
type SimpleFilter struct {
	Hide map[int]bool
}

// Print decides if r is to be printed.
func (f *SimpleFilter) Print(r Response) bool {
	if r.HTTPResponse == nil {
		return true
	}
	return !f.Hide[r.HTTPResponse.StatusCode]
}
