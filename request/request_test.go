package request

import (
	"io/ioutil"
	"net/http"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

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
		Header   Header
		Body     string
		Template string
		Value    string
		Checks   []CheckFunc
	}{
		// basic URL tests
		{
			URL: "https://www.example.com",
			Checks: []CheckFunc{
				checkURL("https://www.example.com"),
				checkMethod("GET"),
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
				checkURL("https://foo:bar@www.example.com"),
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
				checkURL("https://foobar:secret@www.example.com"),
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
				checkURL("https://foo:secbarret@www.example.com"),
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
				checkURL("https://www.example.com"),
				checkMethod("GET"),
				checkHeader("User-Agent", "monsoon"),
				checkHeader("Accept", "*/*"),
			},
		},
		{
			URL: "https://www.example.com",
			Header: Header{
				"User-Agent": []string{"foobar"},
			},
			Checks: []CheckFunc{
				checkURL("https://www.example.com"),
				checkMethod("GET"),
				checkHeader("User-Agent", "foobar"),
			},
		},
		{
			URL: "https://www.example.com",
			Header: Header{
				"User-Agent": []string{"fooFUZZbar"},
			},
			Value: "xxxx",
			Checks: []CheckFunc{
				checkURL("https://www.example.com"),
				checkMethod("GET"),
				checkHeader("User-Agent", "fooxxxxbar"),
			},
		},
		{
			URL: "https://www.example.com",
			Header: Header{
				"User-Agent": []string{"foo", "bar"},
			},
			Value: "xxxx",
			Checks: []CheckFunc{
				checkURL("https://www.example.com"),
				checkMethod("GET"),
				checkHeaderMulti("User-Agent", []string{"foo", "bar"}),
			},
		},
		// methods
		{
			URL:    "https://www.example.com",
			Method: "POST",
			Checks: []CheckFunc{
				checkURL("https://www.example.com"),
				checkMethod("POST"),
			},
		},
		{
			URL:    "https://www.example.com",
			Method: "POSTFUZZ",
			Value:  "xxxx",
			Checks: []CheckFunc{
				checkURL("https://www.example.com"),
				checkMethod("POSTxxxx"),
			},
		},
		{
			URL:    "https://www.example.com",
			Method: "POST",
			Body:   "foobar baz",
			Checks: []CheckFunc{
				checkURL("https://www.example.com"),
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
				checkURL("https://www.example.com"),
				checkMethod("POST"),
				checkBody("foobarxxxxbaz"),
			},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			req := Request{
				URL: test.URL,
				Method: test.Method,
				Header: test.Header,
				Body: test.Body,
			}

			template := "FUZZ"
			if test.Template != "" {
				template = test.Template
			}

			res, err := req.Apply(template, test.Value)
			if err != nil {
				t.Fatal(err)
			}

			if res == nil {
				t.Fatalf("returned *http.Request is nil")
			}

			for _, fn := range test.Checks {
				fn(t, res)
			}
		})
	}
}
