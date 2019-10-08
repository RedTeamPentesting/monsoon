package request

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHeaderSet(t *testing.T) {
	var tests = []struct {
		start  http.Header
		values []string
		item   string
		want   http.Header
	}{
		{
			// this is a default value also contained in DefaultHeader
			start: http.Header{"User-Agent": []string{"monsoon"}},
			// overwrite default value
			values: []string{"user-agent: foobar"},
			want:   http.Header{"User-Agent": []string{"foobar"}},
		},
		{
			start: http.Header{"User-Agent": []string{"monsoon"}},
			// overwrite default value with empty string
			values: []string{"user-agent:"},
			want:   http.Header{"User-Agent": []string{""}},
		},
		{
			start: http.Header{
				"User-Agent": []string{"monsoon"},
				"X-Others":   []string{"out-there"},
			},
			// just header name -> remove header completely
			values: []string{"user-agent"},
			want: http.Header{
				"X-Others": []string{"out-there"},
			},
		},
		{
			start: http.Header{"User-Agent": []string{"monsoon"}},
			values: []string{
				"foo: bar",
				"foo: baz",
				"foo: quux",
			},
			want: http.Header{
				"User-Agent": []string{"monsoon"},
				"Foo":        []string{"bar", "baz", "quux"},
			},
		},
		{
			// make sure that replacing FUZZ in header names still works
			start:  http.Header{"User-Agent": []string{"monsoon"}},
			values: []string{"x-FUZZ: foobar"},
			item:   "testing",
			want: http.Header{
				"User-Agent": []string{"monsoon"},
				"X-Testing":  []string{"foobar"},
			},
		},
		{
			// overwrite Accept header
			start:  DefaultHeader,
			values: []string{"accept: foo"},
			want: http.Header{
				"User-Agent": []string{"monsoon"},
				"Accept":     []string{"foo"},
			},
		},
		{
			// set two values for the Accept header
			start:  DefaultHeader,
			values: []string{"accept: foo", "accept: bar"},
			want: http.Header{
				"User-Agent": []string{"monsoon"},
				"Accept":     []string{"foo", "bar"},
			},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			hdr := NewHeader(test.start)
			for _, v := range test.values {
				_ = hdr.Set(v)
			}

			insertValue := func(s string) string {
				return replaceTemplate(s, "FUZZ", test.item)
			}

			res := make(http.Header)
			hdr.Apply(res, insertValue)

			if !cmp.Equal(test.want, res) {
				t.Errorf("want:\n  %v\ngot:\n  %v", test.want, res)
			}
		})
	}
}

// CheckFunc is one test for an http request generated
type CheckFunc func(testing.TB, *http.Request)

func checkURL(url string) CheckFunc {
	return func(t testing.TB, req *http.Request) {
		if req.URL.String() != url {
			t.Errorf("wrong URL, want %q, got %q", url, req.URL.String())
		}
	}
}

func checkMethod(method string) CheckFunc {
	return func(t testing.TB, req *http.Request) {
		if req.Method != method {
			t.Errorf("wrong method, want %q, got %q", method, req.Method)
		}
	}
}

func checkHeader(name, value string) CheckFunc {
	name = textproto.CanonicalMIMEHeaderKey(name)
	return func(t testing.TB, req *http.Request) {
		v, ok := req.Header[name]
		if !ok {
			t.Errorf("required header %q not found", name)
			return
		}

		if len(v) != 1 {
			t.Errorf("more than one value found for header %v: %q", name, v)
		}

		if v[0] != value {
			t.Errorf("wrong value for header %v, want %q, got %q", name, value, v[0])
		}
	}
}

func checkHost(value string) CheckFunc {
	return func(t testing.TB, req *http.Request) {
		if req.Host != value {
			t.Errorf("invalid value for host: want %q, got %q", value, req.Host)
		}
	}
}

func checkHeaderMulti(name string, values []string) CheckFunc {
	name = textproto.CanonicalMIMEHeaderKey(name)
	return func(t testing.TB, req *http.Request) {
		v, ok := req.Header[name]
		if !ok {
			t.Errorf("required header %q not found", name)
			return
		}

		if len(v) != len(values) {
			t.Errorf("wrong number of headers found, want %v, got %v", len(values), len(v))
		}

		sort.Strings(v)
		sort.Strings(values)

		if !cmp.Equal(values, v) {
			t.Error(cmp.Diff(values, v))
		}
	}
}

func checkHeaderAbsent(name string) CheckFunc {
	name = textproto.CanonicalMIMEHeaderKey(name)
	return func(t testing.TB, req *http.Request) {
		v, ok := req.Header[name]
		if ok {
			t.Errorf("header %q (%q) is present (but should not be)", name, v)
			return
		}
	}
}

func checkBasicAuth(user, password string) CheckFunc {
	return func(t testing.TB, req *http.Request) {
		u, p, ok := req.BasicAuth()
		if !ok {
			t.Error("basic auth requested but not present")
			return
		}

		if u != user {
			t.Errorf("wrong username for basic auth: want %q, got %q", user, u)
		}

		if p != password {
			t.Errorf("wrong password for basic auth: want %q, got %q", password, p)
		}
	}
}

func checkBody(body string) CheckFunc {
	return func(t testing.TB, req *http.Request) {
		buf, err := ioutil.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}

		if string(buf) != body {
			t.Errorf("wrong body returned, want:\n  %q\ngot:\n  %q", body, string(buf))
		}
	}
}

func TestRequestApply(t *testing.T) {
	var tests = []struct {
		URL  string
		File string

		Method string
		Header []string // passed in as a sequence of "name: value" strings
		Body   string

		Template             string
		Value                string
		ForceChunkedEncoding bool
		Checks               []CheckFunc
	}{
		// basic URL tests
		{
			URL: "http://www.example.com",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
			},
		},
		{
			// set some headers, including User-Agent
			URL: "http://www.example.com",
			File: `GET /?x=y HTTP/1.1
User-Agent: Firefox
Accept: application/json
X-foo: bar

`,
			Checks: []CheckFunc{
				checkURL("/?x=y"),
				checkMethod("GET"),
				checkHeader("User-Agent", "Firefox"),
				checkHeader("Accept", "application/json"),
				checkHeader("x-foo", "bar"),
				checkBody(""),
			},
		},
		{
			// replace FUZZ in path with empty string
			URL: "http://www.example.com",
			File: `GET /FUZZ HTTP/1.1
User-Agent: Firefox

`,
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("Accept", "*/*"),
			},
		},
		{
			URL: "http://www.example.com/FUZZ",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("Accept", "*/*"),
			},
		},
		{
			// replace FUZZ in path with value
			URL: "http://www.example.com",
			File: `GET /FUZZ HTTP/1.1
User-Agent: Firefox
Accept: */*

`,
			Value: "foobar",
			Checks: []CheckFunc{
				checkURL("/foobar"),
				checkMethod("GET"),
			},
		},
		{
			URL:   "http://www.example.com/FUZZ",
			Value: "foobar",
			Checks: []CheckFunc{
				checkURL("/foobar"),
				checkMethod("GET"),
			},
		},
		{
			// replace value for Host header with target URL
			URL: "http://www.example.com:8443",
			File: `GET / HTTP/1.1
Host: www2.example.com:8888

`,
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("Accept", "*/*"),
			},
		},
		{
			// host name is taken from the target URL, regardless of what's in
			// the template
			URL: "http://www.example.com",
			File: `GET /secret HTTP/1.1
Host: other.com

`,
			Checks: []CheckFunc{
				checkURL("/secret"),
				checkMethod("GET"),
			},
		},
		// basic auth
		{
			// if supplied in the target URL, use that
			URL: "http://foo:bar@www.example.com",
			File: `GET /secret HTTP/1.1
Host: other.com
Authorization: Basic Zm9vOnp6eg==

`,
			Checks: []CheckFunc{
				checkURL("/secret"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
				checkHeader("Authorization", "Basic Zm9vOmJhcg=="),
				checkBasicAuth("foo", "bar"),
			},
		},
		{
			URL: "http://foo:bar@www.example.com",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
				checkBasicAuth("foo", "bar"),
			},
		},
		{
			// if not supplied in the target URL, use the header
			URL: "http://www.example.com",
			File: `GET /secret HTTP/1.1
Host: other.com
Authorization: Basic Zm9vOnp6eg==

`,
			Checks: []CheckFunc{
				checkURL("/secret"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
				checkHeader("Authorization", "Basic Zm9vOnp6eg=="),
				checkBasicAuth("foo", "zzz"),
			},
		},
		{
			URL: "http://fooFUZZ:secret@www.example.com",
			File: `GET /secret HTTP/1.1

`,
			Value: "bar",
			Checks: []CheckFunc{
				checkURL("/secret"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
				checkBasicAuth("foobar", "secret"),
			},
		},
		{
			URL: "http://foo:secFUZZret@www.example.com",
			File: `GET /secret HTTP/1.1

`,
			Value: "bar",
			Checks: []CheckFunc{
				checkURL("/secret"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
				checkBasicAuth("foo", "secbarret"),
			},
		},
		// header tests
		{
			URL:  "http://www.example.com",
			File: "GET / HTTP/1.1\n\n",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
			},
		},
		{
			URL: "http://www.example.com",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
			},
		},
		{
			URL:    "http://www.example.com",
			Header: []string{"User-Agent"}, // empty value means remove header
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeaderAbsent("User-Agent"),
			},
		},
		{
			URL:    "http://www.example.com",
			Header: []string{"user-agent"}, // empty value means remove header
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeaderAbsent("User-Agent"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
User-Agent: Firefox

`,
			Header: []string{"User-Agent"}, // empty value means remove header
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeaderAbsent("User-Agent"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
user-agent: Firefox

`,
			Header: []string{"user-agent"}, // empty value means remove header
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeaderAbsent("User-Agent"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
User-Agent: Firefox

`,
			Header: []string{"User-Agent: foobar"},
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "foobar"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
User-Agent: fooFUZZbar

`,
			Value: "xxxx",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "fooxxxxbar"),
			},
		},
		{
			URL:    "http://www.example.com",
			Header: []string{"User-Agent: fooFUZZbar"},
			Value:  "xxxx",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "fooxxxxbar"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
User-Agent: fooFUZZbar

`,
			Header: []string{"User-Agent: testFUZZvalue"},
			Value:  "xxxx",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "testxxxxvalue"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
User-Agent: foobar

`,
			Value:  "xxxx",
			Header: []string{"User-Agent: fooFUZZbar"},
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "fooxxxxbar"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
Accept: foo
Accept: bar

`,
			Value: "xxxx",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeaderMulti("Accept", []string{"foo", "bar"}),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET / HTTP/1.1

`,
			Header: []string{
				"Accept: foo",
				"Accept: bar",
			},
			Value: "xxxx",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeaderMulti("Accept", []string{"foo", "bar"}),
			},
		},
		// methods
		{
			URL: "http://www.example.com",
			File: `POST / HTTP/1.1

`,
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POST"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET / HTTP/1.1

`,
			Method: "POST",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POST"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET / HTTP/1.1

`,
			Method: "POSTFUZZ",
			Value:  "xxxx",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POSTxxxx"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `POSTFUZZ / HTTP/1.1

`,
			Value: "xxxx",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POSTxxxx"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `POST / HTTP/1.1
Content-Length: 80

foobarFUZZbaz`,
			Body:  "otherFUZZvalue",
			Value: "xxxx",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POST"),
				checkBody("otherxxxxvalue"),
				checkHeader("Content-Length", "14"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `POST / HTTP/1.1
Content-Length: 80

foobarFUZZbaz`,
			Value: "xxxx",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POST"),
				checkBody("foobarxxxxbaz"),
				checkHeader("Content-Length", "13"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `POST / HTTP/1.1

`,
			Body:  "foobarFUZZbaz",
			Value: "xxxx",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POST"),
				checkBody("foobarxxxxbaz"),
				checkHeader("Content-Length", "13"),
			},
		},
		{
			// test chunked encoding
			URL:                  "http://www.example.com",
			Method:               "POST",
			Body:                 "foobar",
			ForceChunkedEncoding: true,
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POST"),
				checkBody("foobar"),
				checkHeaderAbsent("Content-Length"),
			},
		},
		{
			// ensure that the Host header is passed on directly and not taken from the target URL
			URL: "http://www.example.com",
			File: `GET /secret HTTP/1.1
Host: server:1234

`,
			Checks: []CheckFunc{
				checkURL("/secret"),
				checkMethod("GET"),
				checkHost("server:1234"),
			},
		},
		{
			// overwrite host header
			URL: "http://www.example.com",
			File: `GET /secret HTTP/1.1
Host: server:1234

`,
			Header: []string{"host: foobar:8888"},
			Checks: []CheckFunc{
				checkURL("/secret"),
				checkMethod("GET"),
				checkHost("foobar:8888"),
			},
		},
		{
			// replace strings in header values
			URL: "http://www.example.com",
			Header: []string{
				"Accept: foo-FUZZ",
				"Accept: other",
			},
			Value: "xxxx",
			Checks: []CheckFunc{
				checkHeaderMulti("Accept", []string{"foo-xxxx", "other"}),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET /secret HTTP/1.1
Host: server:1234

`,
			Header: []string{
				"Accept: foo-FUZZ",
				"Accept: other",
			},
			Value: "xxxx",
			Checks: []CheckFunc{
				checkHeaderMulti("Accept", []string{"foo-xxxx", "other"}),
			},
		},
		{
			URL: "http://www.example.com",
			Header: []string{
				"Host: foo-FUZZ",
			},
			Value: "xxxx",
			Checks: []CheckFunc{
				checkHost("foo-xxxx"),
			},
		},
		{
			// replace strings in header names
			URL:    "http://www.example.com",
			Header: []string{"X-FUZZ: fooboar"},
			Value:  "testheader",
			Checks: []CheckFunc{
				checkHeader("X-testheader", "fooboar"),
			},
		},
		{
			URL: "http://www.example.com",
			File: `GET /secret HTTP/1.1
Host: server:1234

`,
			Header: []string{"X-FUZZ: fooboar"},
			Value:  "testheader",
			Checks: []CheckFunc{
				checkHeader("X-testheader", "fooboar"),
			},
		},
	}

	for _, test := range tests {
		tempdir, err := ioutil.TempDir("", "monsoon-test-request-")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			err := os.RemoveAll(tempdir)
			if err != nil {
				t.Fatal(err)
			}
		}()

		t.Run("", func(t *testing.T) {
			var filename string
			if test.File != "" {
				filename = filepath.Join(tempdir, "test-"+strings.Replace(t.Name(), "/", "_", -1))
				err := ioutil.WriteFile(filename, []byte(test.File), 0644)
				if err != nil {
					t.Fatal(err)
				}

				defer func() {
					err := os.Remove(filename)
					if err != nil {
						t.Fatal(err)
					}
				}()
			}

			req := New(test.Template)
			req.URL = test.URL
			if test.File != "" {
				req.TemplateFile = filename
			}
			req.Method = test.Method
			req.Body = test.Body
			req.ForceChunkedEncoding = test.ForceChunkedEncoding
			for _, hdr := range test.Header {
				err := req.Header.Set(hdr)
				if err != nil {
					t.Fatal(err)
				}
			}

			genReq, err := req.Apply(test.Value)
			if err != nil {
				t.Fatal(err)
			}

			if genReq == nil {
				t.Fatalf("returned *http.Request is nil")
			}

			// run the request against a test server, parse it, then run the tests
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for _, fn := range test.Checks {
					fn(t, r)
				}
			}))
			defer srv.Close()

			srvURL, err := url.Parse(srv.URL)
			if err != nil {
				t.Fatal(err)
			}

			// send the request to the test server
			tr := &http.Transport{
				Dial: func(network, addr string) (net.Conn, error) {
					port := srvURL.Port()
					if port == "" {
						switch srvURL.Scheme {
						case "http":
							port = "80"
						case "https":
							port = "443"
						default:
							panic("unknown scheme " + srvURL.Scheme)
						}
					}
					testServerAddr := fmt.Sprintf("%v:%v", srvURL.Hostname(), port)
					return net.Dial("tcp", testServerAddr)
				},
			}

			_, err = tr.RoundTrip(genReq)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
