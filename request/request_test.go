package request

import (
	"fmt"
	"io"
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
	tests := []struct {
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
		buf, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}

		if string(buf) != body {
			t.Errorf("wrong body returned, want:\n  %q\ngot:\n  %q", body, string(buf))
		}
	}
}

func TestRequestApply(t *testing.T) {
	tests := []struct {
		URL  string
		File string

		Method string
		Header []string // passed in as a sequence of "name: value" strings
		Body   string

		Names                []string
		Values               []string
		ForceChunkedEncoding bool
		Checks               []CheckFunc
	}{
		// basic URL tests
		{
			// replace nothing
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
			Names:  []string{"FUZZ"},
			Values: []string{""},
			URL:    "http://www.example.com",
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
			Names:  []string{"FUZZ"},
			Values: []string{""},
			URL:    "http://www.example.com/FUZZ",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("Accept", "*/*"),
			},
		},
		{
			// replace FUZZ in path with value
			Names:  []string{"FUZZ"},
			Values: []string{"foobar"},
			URL:    "http://www.example.com",
			File: `GET /FUZZ HTTP/1.1
User-Agent: Firefox
Accept: */*

`,
			Checks: []CheckFunc{
				checkURL("/foobar"),
				checkMethod("GET"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"foobar"},
			URL:    "http://www.example.com/FUZZ",
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
			Names:  []string{"ZZZZ"},
			Values: []string{"bar"},
			URL:    "http://fooZZZZ:secret@www.example.com",
			File: `GET /secret HTTP/1.1

`,
			Checks: []CheckFunc{
				checkURL("/secret"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
				checkBasicAuth("foobar", "secret"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"bar"},
			URL:    "http://foo:secFUZZret@www.example.com",
			File: `GET /secret HTTP/1.1

`,
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
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
User-Agent: fooFUZZbar

`,
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "fooxxxxbar"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL:    "http://www.example.com",
			Header: []string{"User-Agent: fooFUZZbar"},

			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "fooxxxxbar"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
User-Agent: fooFUZZbar

`,
			Header: []string{"User-Agent: testFUZZvalue"},
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "testxxxxvalue"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
User-Agent: foobar

`,
			Header: []string{"User-Agent: fooFUZZbar"},
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "fooxxxxbar"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `GET / HTTP/1.1
Accept: foo
Accept: bar

`,
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("GET"),
				checkHeaderMulti("Accept", []string{"foo", "bar"}),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `GET / HTTP/1.1

`,
			Header: []string{
				"Accept: foo",
				"Accept: bar",
			},
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
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `GET / HTTP/1.1

`,
			Method: "POSTFUZZ",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POSTxxxx"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `POSTFUZZ / HTTP/1.1

`,
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POSTxxxx"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `POST / HTTP/1.1
Content-Length: 80

foobarFUZZbaz`,
			Body: "otherFUZZvalue",
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POST"),
				checkBody("otherxxxxvalue"),
				checkHeader("Content-Length", "14"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `POST / HTTP/1.1
Content-Length: 80

foobarFUZZbaz`,
			Checks: []CheckFunc{
				checkURL("/"),
				checkMethod("POST"),
				checkBody("foobarxxxxbaz"),
				checkHeader("Content-Length", "13"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `POST / HTTP/1.1

`,
			Body: "foobarFUZZbaz",
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
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			Header: []string{
				"Accept: foo-FUZZ",
				"Accept: other",
			},
			Checks: []CheckFunc{
				checkHeaderMulti("Accept", []string{"foo-xxxx", "other"}),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			File: `GET /secret HTTP/1.1
Host: server:1234

`,
			Header: []string{
				"Accept: foo-FUZZ",
				"Accept: other",
			},
			Checks: []CheckFunc{
				checkHeaderMulti("Accept", []string{"foo-xxxx", "other"}),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"xxxx"},

			URL: "http://www.example.com",
			Header: []string{
				"Host: foo-FUZZ",
			},
			Checks: []CheckFunc{
				checkHost("foo-xxxx"),
			},
		},
		{
			// replace strings in header names
			Names:  []string{"FUZZ"},
			Values: []string{"testheader"},

			URL:    "http://www.example.com",
			Header: []string{"X-FUZZ: fooboar"},
			Checks: []CheckFunc{
				checkHeader("X-testheader", "fooboar"),
			},
		},
		{
			Names:  []string{"FUZZ"},
			Values: []string{"testheader"},

			URL: "http://www.example.com",
			File: `GET /secret HTTP/1.1
Host: server:1234

`,
			Header: []string{"X-FUZZ: fooboar"},
			Checks: []CheckFunc{
				checkHeader("X-testheader", "fooboar"),
			},
		},
		// replace multiple values
		{
			Names:  []string{"DIR", "FILE"},
			Values: []string{"foo", "bar"},
			URL:    "http://www.example.com/include/DIR/FILE",
			Checks: []CheckFunc{
				checkURL("/include/foo/bar"),
				checkMethod("GET"),
			},
		},
		{
			Names:  []string{"METHOD", "FUZZ"},
			Values: []string{"DELETE", "testfile"},

			URL:    "http://www.example.com/FUZZ",
			Method: "METHOD",

			Checks: []CheckFunc{
				checkURL("/testfile"),
				checkMethod("DELETE"),
			},
		},
	}

	for _, test := range tests {
		tempdir, err := os.MkdirTemp("", "monsoon-test-request-")
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
				err := os.WriteFile(filename, []byte(test.File), 0o644)
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

			req := New(test.Names)
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

			genReq, err := req.Apply(test.Values)
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

func TestTemplateHTTP2(t *testing.T) {
	tests := []struct {
		input  []byte
		output []byte
	}{
		{
			input:  []byte("GET / HTTP/1.1\nHost: www.example.com\n\n"),
			output: []byte("GET / HTTP/1.1\nHost: www.example.com\n\n"),
		},
		{
			input:  []byte("GET / HTTP/2\nHost: www.example.com\n\n"),
			output: []byte("GET / HTTP/2.0\nHost: www.example.com\n\n"),
		},
		{
			input:  []byte("GET / HTTP/2\r\nHost: www.example.com\r\n\r\n"),
			output: []byte("GET / HTTP/2.0\r\nHost: www.example.com\r\n\r\n"),
		},
		{
			input:  []byte("GET / HTTP/2"),
			output: []byte("GET / HTTP/2.0"),
		},
		{
			input:  []byte("GET / HTTP/2\n"),
			output: []byte("GET / HTTP/2.0\n"),
		},
		{
			input:  []byte("GET / HTTP/2\r\n"),
			output: []byte("GET / HTTP/2.0\r\n"),
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			tempfile := t.TempDir() + "request-template.txt"

			err := os.WriteFile(tempfile, test.input, 0o600)
			if err != nil {
				t.Fatal(err)
			}

			req := &Request{
				TemplateFile: tempfile,
			}

			out, err := req.template()
			if err != nil {
				t.Fatal(err)
			}

			if !cmp.Equal(test.output, out) {
				t.Error(cmp.Diff(test.output, out))
			}
		})
	}
}
