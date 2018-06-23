// Package request contains functions to build an HTTP request from a template.
package request

import (
	"bytes"
	"fmt"
	"net/http"
	"net/textproto"
	"sort"
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
	// get name and value from s
	data := strings.SplitN(s, ":", 2)
	name := data[0]

	if len(data) == 1 {
		// no value specified, this means the header is to be removed
		http.Header(h).Del(name)
		return nil
	}

	// otherwise we have a name: value pair
	val := data[1]

	// if the header is still at the default value, remove the default value first
	if headerDefaultValue(h, name) {
		http.Header(h).Del(name)
	}

	// strip the leading space if necessary
	if len(val) > 0 && val[0] == ' ' {
		val = val[1:]
	}

	http.Header(h).Add(name, val)
	return nil
}

// Type returns a description string for header.
func (h Header) Type() string {
	return "name: value"
}

func headerDefaultValue(h Header, name string) bool {
	key := textproto.CanonicalMIMEHeaderKey(name)

	v, ok := h[key]
	if !ok {
		return false
	}

	def, ok := DefaultHeader[key]
	if !ok {
		return false
	}

	if len(v) != len(def) {
		return false
	}

	// make copies of the two slices to prevent modifying the original data by
	// sorting
	a := make([]string, len(v))
	copy(a, v)
	sort.Strings(a)

	b := make([]string, len(v))
	copy(b, def)
	sort.Strings(b)

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// DefaultHeader contains all HTTP header values that are added by default. If
// the header is already present, it is not added.
var DefaultHeader = Header{
	"Accept":     []string{"*/*"},
	"User-Agent": []string{"monsoon"},
}

// Request is a template for an HTTP request.
type Request struct {
	URL    string
	Method string
	Header Header
	Body   string

	ForceChunkedEncoding bool
}

// New returns a new request.
func New() *Request {
	hdr := make(http.Header)
	for k, v := range DefaultHeader {
		hdr[k] = v
	}

	return &Request{
		Header: Header(hdr),
	}
}

// AddFlags adds flags for all options of a request to fs.
func (r *Request) AddFlags(fs *pflag.FlagSet) {
	// basics
	fs.StringVar(&r.Method, "request", "GET", "use HTTP request `method`")
	fs.MarkDeprecated("request", "use --method")
	fs.StringVarP(&r.Method, "method", "X", "GET", "use HTTP request `method`")
	fs.StringVarP(&r.Body, "data", "d", "", "transmit `data` in the HTTP request body")
	fs.VarP(r.Header, "header", "H", "add `\"name: value\"` as an HTTP request header")

	// configure request
	fs.BoolVar(&r.ForceChunkedEncoding, "force-chunked-encoding", false, `do not set the Content-Length HTTP header and use chunked encoding`)
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
	body := []byte(insertValue(r.Body))
	req, err := http.NewRequest(insertValue(r.Method), url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	if r.ForceChunkedEncoding {
		req.ContentLength = -1
	}

	if req.URL.User != nil {
		u := req.URL.User.Username()
		p, _ := req.URL.User.Password()
		req.SetBasicAuth(u, p)
	}

	// make sure there's a valid path
	if req.URL.Path == "" {
		req.URL.Path = "/"
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

	// special handling for sending a request without any user-agent header:
	// it must be set to the empty string in the http.Request.Header to prevent
	// Go stdlib from setting the default user agent
	if _, ok := r.Header["User-Agent"]; !ok {
		req.Header.Set("User-Agent", "")
	}

	// known limitation: due to the way the Go stdlib handles setting the
	// user-agent header, it's currently not possible to send a request with
	// multiple user-agent headers.

	return req, nil
}
