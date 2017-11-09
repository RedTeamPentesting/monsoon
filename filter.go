package main

// Filter decides whether to reject a Response.
type Filter interface {
	Reject(Response) bool
}

// FilterStatusCode hides responses based on the HTTP status code.
type FilterStatusCode struct {
	status map[int]bool
}

// NewFilterStatusCode returns a filter based on HTTP status code.
func NewFilterStatusCode(rejects []int) FilterStatusCode {
	f := FilterStatusCode{
		status: make(map[int]bool, len(rejects)),
	}
	for _, code := range rejects {
		f.status[code] = true
	}
	return f
}

// Reject decides if r is to be printed.
func (f FilterStatusCode) Reject(r Response) bool {
	if r.HTTPResponse == nil {
		return false
	}
	return f.status[r.HTTPResponse.StatusCode]
}
