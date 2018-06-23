package request

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHeaderSet(t *testing.T) {
	var tests = []struct {
		start  Header
		values []string
		want   Header
	}{
		{
			// this is a default value also contained in DefaultHeader
			start: Header{"User-Agent": []string{"monsoon"}},
			// overwrite default value
			values: []string{"user-agent: foobar"},
			want:   Header{"User-Agent": []string{"foobar"}},
		},
		{
			start: Header{"User-Agent": []string{"monsoon"}},
			// overwrite default value with empty string
			values: []string{"user-agent:"},
			want:   Header{"User-Agent": []string{""}},
		},
		{
			start: Header{
				"User-Agent": []string{"monsoon"},
				"X-Others":   []string{"out-there"},
			},
			// just header name -> remove header completely
			values: []string{"user-agent"},
			want: Header{
				"X-Others": []string{"out-there"},
			},
		},
		{
			start: Header{"User-Agent": []string{"monsoon"}},
			values: []string{
				"foo: bar",
				"foo: baz",
				"foo: quux",
			},
			want: Header{
				"User-Agent": []string{"monsoon"},
				"Foo":        []string{"bar", "baz", "quux"},
			},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			h := test.start
			for _, v := range test.values {
				h.Set(v)
			}

			if !cmp.Equal(test.want, h) {
				t.Errorf("want:\n  %v\ngot:\n  %v", test.want, h)
			}
		})
	}
}

func TestRequestApply(t *testing.T) {
	// CheckFunc is one test for an http request generated
	type CheckFunc func(testing.TB, *http.Request)

	checkURL := func(url string) CheckFunc {
		return func(t testing.TB, req *http.Request) {
			if req.URL.String() != url {
				t.Errorf("wrong URL, want %q, got %q", url, req.URL.String())
			}
		}
	}

	checkMethod := func(method string) CheckFunc {
		return func(t testing.TB, req *http.Request) {
			if req.Method != method {
				t.Errorf("wrong method, want %q, got %q", method, req.Method)
			}
		}
	}

	checkHeader := func(name, value string) CheckFunc {
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

	checkHeaderMulti := func(name string, values []string) CheckFunc {
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

	checkHeaderAbsent := func(name string) CheckFunc {
		return func(t testing.TB, req *http.Request) {
			v, ok := req.Header[name]
			if ok {
				t.Errorf("header %q (%q) is present (but should not be)", name, v)
				return
			}
		}
	}

	checkBasicAuth := func(user, password string) CheckFunc {
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

	checkBody := func(body string) CheckFunc {
		return func(t testing.TB, req *http.Request) {
			buf, err := ioutil.ReadAll(req.Body)
			if err != nil {
				t.Fatal(err)
			}

			if string(buf) != body {
				t.Errorf("wrong body returned, want:\n  %v\ngot:\n  %v", body, string(buf))
			}
		}
	}

	var tests = []struct {
		URL      string
		Method   string
		Header   []string // passed in as a sequence of "name: value" strings
		Body     string
		Template string
		Value    string
		Checks   []CheckFunc
	}{
		// basic URL tests
		{
			URL: "https://www.example.com",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
			},
		},
		{
			URL: "https://www.example.com/FUZZ",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("GET"),
			},
		},
		{
			URL:   "https://www.example.com/FUZZ",
			Value: "foobar",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/foobar"),
				checkMethod("GET"),
			},
		},
		{
			URL:      "https://www.example.com/xxx",
			Template: "xx",
			Value:    "foobar",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/foobarx"),
				checkMethod("GET"),
			},
		},
		{
			URL:      "https://www.example.com/xxx",
			Template: "xx",
			Value:    "foobar",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/foobarx"),
				checkMethod("GET"),
			},
		},
		// basic auth
		{
			URL: "https://foo:bar@www.example.com",
			Checks: []CheckFunc{
				checkURL("https://foo:bar@www.example.com/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
				checkBasicAuth("foo", "bar"),
			},
		},
		{
			URL:   "https://fooFUZZ:secret@www.example.com",
			Value: "bar",
			Checks: []CheckFunc{
				checkURL("https://foobar:secret@www.example.com/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
				checkBasicAuth("foobar", "secret"),
			},
		},
		{
			URL:   "https://foo:secFUZZret@www.example.com",
			Value: "bar",
			Checks: []CheckFunc{
				checkURL("https://foo:secbarret@www.example.com/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
				checkBasicAuth("foo", "secbarret"),
			},
		},
		// header tests
		{
			URL: "https://www.example.com",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
			},
		},
		{
			URL:    "https://www.example.com",
			Header: []string{"User-Agent"}, // empty value means remove header
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("GET"),
				checkHeaderAbsent("User-Agent"),
			},
		},
		{
			URL:    "https://www.example.com",
			Header: []string{"User-Agent: foobar"},
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "foobar"),
			},
		},
		{
			URL:    "https://www.example.com",
			Header: []string{"User-Agent: fooFUZZbar"},
			Value:  "xxxx",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("GET"),
				checkHeader("User-Agent", "fooxxxxbar"),
			},
		},
		{
			URL: "https://www.example.com",
			Header: []string{
				"Accept: foo",
				"Accept: bar",
			},
			Value: "xxxx",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("GET"),
				checkHeaderMulti("Accept", []string{"foo", "bar"}),
			},
		},
		// methods
		{
			URL:    "https://www.example.com",
			Method: "POST",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("POST"),
			},
		},
		{
			URL:    "https://www.example.com",
			Method: "POSTFUZZ",
			Value:  "xxxx",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("POSTxxxx"),
			},
		},
		{
			URL:    "https://www.example.com",
			Method: "POST",
			Body:   "foobar baz",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("POST"),
				checkBody("foobar baz"),
			},
		},
		{
			URL:    "https://www.example.com",
			Method: "POST",
			Body:   "foobarFUZZbaz",
			Value:  "xxxx",
			Checks: []CheckFunc{
				checkURL("https://www.example.com/"),
				checkMethod("POST"),
				checkBody("foobarxxxxbaz"),
			},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			req := New()
			req.URL = test.URL
			req.Method = test.Method
			req.Body = test.Body
			for _, hdr := range test.Header {
				err := req.Header.Set(hdr)
				if err != nil {
					t.Fatal(err)
				}
			}

			template := "FUZZ"
			if test.Template != "" {
				template = test.Template
			}

			genReq, err := req.Apply(template, test.Value)
			if err != nil {
				t.Fatal(err)
			}

			if genReq == nil {
				t.Fatalf("returned *http.Request is nil")
			}

			// dump the request and parse it again, then run the tests
			buf, err := httputil.DumpRequestOut(genReq, true)
			if err != nil {
				t.Fatal(err)
			}

			parsedReq, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(buf)))
			if err != nil {
				t.Fatal(err)
			}

			// fill in URL details that were lost in transit
			parsedReq.URL.Host = genReq.URL.Host
			parsedReq.URL.Scheme = genReq.URL.Scheme
			parsedReq.URL.User = genReq.URL.User

			for _, fn := range test.Checks {
				fn(t, parsedReq)
			}
		})
	}
}
