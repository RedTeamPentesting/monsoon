// Package request contains functions to build an HTTP request from a template.
package request

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/pflag"
)

// Header is an HTTP header that implements the pflag.Value interface.
type Header http.Header

func (h Header) String() (s string) {
	for k, v := range h {
		s += fmt.Sprintf(`"%v: %v", `, k, strings.Join(v, ","))
	}

	// if there's at least one value, strip the extra ", " from the end of the string
	if len(s) > 0 {
		s = strings.TrimSuffix(s, ", ")
	}
	return s
}

// Set allows setting an HTTP header via options and pflag.
func (h Header) Set(s string) error {
	if !strings.ContainsAny(s, ":") {
		return fmt.Errorf("invalid format for HTTP header, need `name: value`: %q", s)
	}

	// get name and value from s
	data := strings.SplitN(s, ":", 2)
	name := data[0]
	var val string
	// set val to the value if some was passed
	if len(data) > 1 {
		val = data[1]
		// strip the leading space if necessary
		if len(val) > 0 && val[0] == ' ' {
			val = val[1:]
		}
	}

	http.Header(h).Add(name, val)
	return nil
}

// Type returns a description string for header.
func (h Header) Type() string {
	return "name: value"
}

// Request is a template for an HTTP request.
type Request struct {
	URL    string
	Method string
	Header Header
	Body   string
}

// New returns a new request.
func New() *Request {
	hdr := make(http.Header)

	// set default header
	hdr.Set("Accept", "*/*")
	hdr.Set("User-Agent", "monsoon")

	return &Request{
		Header: Header(hdr),
	}
}

// AddFlags adds flags for all options of a request to fs.
func (r *Request) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.Method, "request", "GET", "use HTTP request `method`")
	fs.MarkDeprecated("request", "use --method")
	fs.StringVarP(&r.Method, "method", "X", "GET", "use HTTP request `method`")

	fs.StringVarP(&r.Body, "data", "d", "", "transmit `data` in the HTTP request body")
	fs.VarP(r.Header, "header", "H", "add `\"name: value\"` as an HTTP request header")
}

func replaceTemplate(s, template, value string) string {
	if !strings.Contains(s, template) {
		return s
	}

	return strings.Replace(s, template, value, -1)
}

// Apply replaces the template with value in all fields of the request and
// returns a new http.Request.
func (r *Request) Apply(template, value string) (*http.Request, error) {
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
