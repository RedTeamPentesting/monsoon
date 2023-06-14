// Package request contains functions to build an HTTP request from a template.
package request

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
)

// Header is an HTTP header that implements the pflag.Value interface.
type Header struct {
	Header http.Header
	Remove map[string]struct{} // entries are to be removed before sending the HTTP request
}

func (h Header) String() (s string) {
	for k, v := range h.Header {
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
		h.Remove[name] = struct{}{}
		return nil
	}

	// otherwise we have a name: value pair
	val := data[1]

	// if the header is still at the default value, remove the default value first
	if headerDefaultValue(h, name) {
		delete(h.Header, textproto.CanonicalMIMEHeaderKey(name))
	}

	// strip the leading space if necessary
	if len(val) > 0 && val[0] == ' ' {
		val = val[1:]
	}

	// use original name in case there's a string we need to replace later
	h.Header[name] = append(h.Header[name], val)

	return nil
}

// Type returns a description string for header.
func (h Header) Type() string {
	return "name: value"
}

// NewHeader initializes a Header.
func NewHeader(defaults http.Header) *Header {
	hdr := make(http.Header)
	for k, vs := range defaults {
		hdr[k] = vs
	}

	return &Header{
		Header: hdr,
		Remove: make(map[string]struct{}),
	}
}

// Apply applies the values in h to the target http.Header. The function
// insertValue is called for all names and values before adding them.
func (h Header) Apply(hdr http.Header, insertValue func(string) string) {
	for k, vs := range h.Header {
		// don't set the header if it is already set in the request and the
		// value is the default one.
		if _, ok := hdr[k]; ok && headerDefaultValue(h, k) {
			continue
		}

		// remove value if present
		hdr.Del(k)

		// add values
		k = insertValue(k)

		for _, v := range vs {
			hdr.Add(k, insertValue(v))
		}
	}

	for k := range h.Remove {
		hdr.Del(k)
	}
}

func headerDefaultValue(h Header, name string) bool {
	key := textproto.CanonicalMIMEHeaderKey(name)

	v, ok := h.Header[key]
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
var DefaultHeader = http.Header{
	"Accept":     []string{"*/*"},
	"User-Agent": []string{"monsoon"},
}

// Request is a template for an HTTP request.
type Request struct {
	URL    string
	Method string
	Header *Header
	Body   string

	UserPass string // user:password for HTTP basic auth

	TemplateFile           string // used to read the request from a file
	internalTemplateBuffer []byte
	templateMutex          sync.Mutex

	Names []string // these string are being replaced by a value in a specific http request

	ForceChunkedEncoding bool
}

// New returns a new request.
func New(names []string) *Request {
	return &Request{
		Header: NewHeader(DefaultHeader),
		Names:  names,
	}
}

func (r *Request) template() ([]byte, error) {
	r.templateMutex.Lock()
	defer r.templateMutex.Unlock()

	if r.TemplateFile == "" {
		return nil, nil
	}

	if r.internalTemplateBuffer == nil {
		buf, err := os.ReadFile(r.TemplateFile)
		if err != nil {
			return nil, err
		}

		// the Go http library is unable to parse "HTTP/2", so we replace it by "HTTP/2.0"
		// see https://stackoverflow.com/a/62388961

		firstLine, rest, linefeedFound := bytes.Cut(buf, []byte("\n"))

		if !linefeedFound {
			// consider everything as the first line
			firstLine = buf
			rest = nil
		}

		suffix := []byte("HTTP/2")
		replacement := []byte("HTTP/2.0")

		// handle additional CR (line ending is \r\n)
		if bytes.HasSuffix(firstLine, []byte("\r")) {
			suffix = append(suffix, '\r')
			replacement = append(replacement, '\r')
		}

		if bytes.HasSuffix(firstLine, suffix) {
			firstLine = bytes.TrimSuffix(firstLine, []byte(suffix))

			// build new buffer so we can squezee the longer text in
			newbuf := make([]byte, 0, len(buf)+2)
			newbuf = append(newbuf, firstLine...)
			newbuf = append(newbuf, replacement...)

			firstLine = newbuf
		}

		// put it back together
		if linefeedFound {
			firstLine = append(firstLine, '\n')
			firstLine = append(firstLine, rest...)
		}

		r.internalTemplateBuffer = firstLine
	}

	return r.internalTemplateBuffer, nil
}

func replaceTemplate(s, template, value string) string {
	if !strings.Contains(s, template) {
		return s
	}

	return strings.Replace(s, template, value, -1)
}

func requestFromBuffer(buf []byte, target *url.URL, replace func([]byte) []byte) (*http.Request, error) {
	// replace the placeholder in the file we just read
	if replace != nil {
		buf = replace(buf)
	}

	rd := bufio.NewReader(bytes.NewReader(buf))
	req, err := http.ReadRequest(rd)
	if err != nil {
		return nil, fmt.Errorf("parse HTTP request: %w", err)
	}

	// append the rest of the file to the body
	rest, err := io.ReadAll(rd)
	if err == io.EOF {
		// if nothing further can be read, that's fine with us
		err = nil
	}
	if err != nil {
		return nil, err
	}

	// rebuild body
	origBody, err := io.ReadAll(req.Body)
	if err == io.ErrUnexpectedEOF {
		err = nil
	}
	if err != nil {
		return nil, err
	}

	origBody = append(origBody, rest...)
	req.Body = io.NopCloser(bytes.NewReader(origBody))
	req.ContentLength = int64(len(origBody))

	// fill some details from the URL

	// check that the URL does not contain too much information, only host,
	// port, and scheme are considered
	if target.Path != "" && target.Path != "/" {
		return nil, errors.New("URL must not contain a path, it's taken from the template file")
	}

	if target.RawQuery != "" {
		return nil, errors.New("URL must not contain a query string, it's taken from the template file")
	}

	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host

	// RequestURI must be empty for client requests
	req.RequestURI = ""

	if target.User != nil {
		req.URL.User = target.User
	}

	return req, nil
}

func (r *Request) TargetURL() (string, error) {
	if r.TemplateFile == "" {
		return r.URL, nil
	}

	target, err := url.Parse(r.URL)
	if err != nil {
		return r.URL, nil
	}

	tmpl, err := r.template()
	if err != nil {
		return "", fmt.Errorf("read request template: %w", err)
	}

	req, err := requestFromBuffer(tmpl, target, nil)
	if err != nil {
		return "", fmt.Errorf("parse request template: %w", err)
	}

	return req.URL.String(), nil
}

// Apply replaces the template with value in all fields of the request and
// returns a new http.Request.
func (r *Request) Apply(values []string) (*http.Request, error) {
	if len(r.Names) != len(values) {
		return nil, fmt.Errorf("got %v values for %v replacement strings (%v)", len(values), len(r.Names), strings.Join(r.Names, ", "))
	}

	insertValues := func(s string) string {
		for i := range r.Names {
			s = replaceTemplate(s, r.Names[i], values[i])
		}

		return s
	}

	insertValueBytes := func(buf []byte) []byte {
		for i := range r.Names {
			buf = bytes.Replace(buf, []byte(r.Names[i]), []byte(values[i]), -1)
		}

		return buf
	}

	targetURL := insertValues(r.URL)
	body := []byte(insertValues(r.Body))

	var req *http.Request

	// if a template file is given, read the HTTP request from it as a basis
	if r.TemplateFile != "" {
		target, err := url.Parse(targetURL)
		if err != nil {
			return nil, err
		}

		tmpl, err := r.template()
		if err != nil {
			return nil, fmt.Errorf("read request template: %w", err)
		}

		req, err = requestFromBuffer(tmpl, target, insertValueBytes)
		if err != nil {
			return nil, fmt.Errorf("parse request template: %w", err)
		}

		if len(body) > 0 {
			// use new body and set content length
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(len(body))
		}

		if r.Method != "" {
			req.Method = insertValues(r.Method)
		}

	} else {
		var err error

		// create new request from scratch
		req, err = http.NewRequest(insertValues(r.Method), targetURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
	}

	if r.ForceChunkedEncoding {
		req.ContentLength = -1
	}

	// if the URL has user and password, use that
	if req.URL.User != nil {
		u := req.URL.User.Username()
		p, _ := req.URL.User.Password()
		req.SetBasicAuth(u, p)
	}

	// but if an explicit user and pass is specified, override them again
	if r.UserPass != "" {
		data := strings.SplitN(insertValues(r.UserPass), ":", 2)
		u := data[0]
		p := ""
		if len(data) > 1 {
			p = data[1]
		}

		req.SetBasicAuth(u, p)
	}

	// make sure there's a valid path
	if req.URL.Path == "" {
		req.URL.Path = "/"
	}

	// apply template headers
	r.Header.Apply(req.Header, insertValues)

	// special handling for the Host header, which needs to be set on the
	// request field Host
	for k, v := range r.Header.Header {
		if textproto.CanonicalMIMEHeaderKey(k) == "Host" {
			req.Host = insertValues(v[0])
		}
	}

	for k := range r.Header.Remove {
		name := textproto.CanonicalMIMEHeaderKey(k)

		// special handling for sending a request without any user-agent header:
		// it must be set to the empty string in the http.Request.Header to prevent
		// Go stdlib from setting the default user agent
		if name == "User-Agent" {
			req.Header.Set("User-Agent", "")
		}

		// known limitation: due to the way the Go stdlib handles setting the
		// user-agent header, it's currently not possible to send a request with
		// multiple user-agent headers.

		// special handling if the Host header is to be removed
		if name == "Host" {
			return nil, errors.New("request without Host header is not supported")
		}
	}

	return req, nil
}

// Target returns the host and port for the request.
func Target(req *http.Request) (host, port string, err error) {
	port = req.URL.Port()
	if port == "" {
		// fill in default ports
		switch req.URL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", "", fmt.Errorf("unknown URL scheme %q", req.URL.Scheme)
		}
	}

	return req.URL.Hostname(), port, nil
}
