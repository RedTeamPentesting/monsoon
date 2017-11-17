package main

import (
	"strconv"
	"strings"
)

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

// parseSizeFilterSpec returns a function that returns true if the size matches with the spec.
//
// possible matches:
//  * exact: 1234
//  * range: 100-200
//  * open range: -200, 200-
func parseSizeFilterSpec(spec string) (func(int) bool, error) {
	if strings.HasPrefix(spec, "-") {
		v, err := strconv.Atoi(spec[1:])
		if err != nil {
			return nil, err
		}

		f := func(size int) bool {
			return size <= v
		}
		return f, nil
	}

	if strings.HasSuffix(spec, "-") {
		v, err := strconv.Atoi(spec[:len(spec)-1])
		if err != nil {
			return nil, err
		}

		f := func(size int) bool {
			return size >= v
		}
		return f, nil
	}

	pos := strings.IndexByte(spec, '-')
	if pos >= 0 {
		s1, s2 := spec[:pos], spec[pos+1:]

		v1, err := strconv.Atoi(s1)
		if err != nil {
			return nil, err
		}

		v2, err := strconv.Atoi(s2)
		if err != nil {
			return nil, err
		}

		f := func(size int) bool {
			return size >= v1 && size <= v2
		}
		return f, nil
	}

	v, err := strconv.Atoi(spec)
	if err != nil {
		return nil, err
	}

	f := func(size int) bool {
		return size == v
	}
	return f, nil
}

// FilterSize hides responses based on a size.
type FilterSize struct {
	headerBytes []func(int) bool
	bodyBytes   []func(int) bool
}

// NewFilterSize returns an initialized FilterSize.
func NewFilterSize(headerBytes, bodyBytes []string) (FilterSize, error) {
	fs := FilterSize{}

	for _, spec := range headerBytes {
		f, err := parseSizeFilterSpec(spec)
		if err != nil {
			return FilterSize{}, err
		}

		fs.headerBytes = append(fs.headerBytes, f)
	}

	for _, spec := range bodyBytes {
		f, err := parseSizeFilterSpec(spec)
		if err != nil {
			return FilterSize{}, err
		}

		fs.bodyBytes = append(fs.bodyBytes, f)
	}

	return fs, nil
}

// Reject decides if r is to be printed.
func (f FilterSize) Reject(r Response) bool {
	for _, f := range f.headerBytes {
		if f(r.Header.Bytes) {
			return true
		}
	}

	for _, f := range f.bodyBytes {
		if f(r.Body.Bytes) {
			return true
		}
	}

	return false
}
