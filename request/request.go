// Package request contains functions to build an HTTP request from a template.
package request

import (
	"net/http"
	"strings"
)

// Request is a template for an HTTP request.
type Request struct {
	URL    string
	Method string
	Header http.Header
	Body   string
}

func replaceTemplate(s, template, value string) string {
	if !strings.Contains(s, template) {
		return s
	}

	return strings.Replace(s, template, value, -1)
}

// Apply replaces the template with value in all fields of the request and
// returns a new http.Request.
func (r Request) Apply(template, value string) (*http.Request, error) {
	insertValue := func(s string) string {
		return replaceTemplate(s, template, value)
	}

	url := insertValue(r.URL)
	req, err := http.NewRequest(insertValue(r.Method), url, strings.NewReader(insertValue(r.Body)))
	if err != nil {
		return nil, err
	}

	if req.URL.User != nil {
		u := req.URL.User.Username()
		p, _ := req.URL.User.Password()
		req.SetBasicAuth(u, p)
	}

	// set default header
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "monsoon")

	// apply template headers
	for k, vs := range r.Header {
		// remove default value if present
		req.Header.Del(k)

		// add values
		k = insertValue(k)
		for _, v := range vs {
			req.Header.Add(k, insertValue(v))
		}
	}

	return req, nil
}
