package recorder

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/RedTeamPentesting/monsoon/request"
	"github.com/google/go-cmp/cmp"
)

func TestTemplate(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "monsoon-recorder-test-")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err := os.RemoveAll(tempdir)
		if err != nil {
			t.Fatal(err)
		}
	}()

	var tests = []struct {
		request func() *request.Request
		want    Template
	}{
		{
			request: func() *request.Request {
				req := request.New("")
				req.URL = "https://localhost:8443/"
				req.Method = "foo"
				return req
			},
			want: Template{
				URL:    "https://localhost:8443/",
				Method: "foo",
				Header: request.DefaultHeader,
			},
		},
		{
			request: func() *request.Request {
				req := request.New("")
				req.URL = "https://localhost:8443/?bar"
				req.Method = "xFUZZ"
				req.Body = "testbody"
				_ = req.Header.Set("x-foo: bar")
				_ = req.Header.Set("accept: application/json")
				_ = req.Header.Set("accept: image/jpeg")
				return req
			},
			want: Template{
				URL:    "https://localhost:8443/?bar",
				Method: "xFUZZ",
				Body:   "testbody",
				Header: http.Header{
					"User-Agent": []string{"monsoon"},
					"Accept":     []string{"application/json", "image/jpeg"},
					"X-Foo":      []string{"bar"},
				},
			},
		},
		{
			request: func() *request.Request {
				fn := filepath.Join(tempdir, "req-from-file")
				data := []byte(`GET /?x=y HTTP/1.1
User-Agent: Firefox
Accept: application/json
Accept: image/jpeg
X-foo: bar

foobar`)
				err := ioutil.WriteFile(fn, data, 0644)
				if err != nil {
					t.Fatal(err)
				}

				req := request.New("")
				req.TemplateFile = fn
				req.URL = "https://host"
				return req
			},
			want: Template{
				URL:    "https://host/?x=y",
				Method: "GET",
				Body:   "foobar",
				Header: http.Header{
					"User-Agent": []string{"Firefox"},
					"Accept":     []string{"application/json", "image/jpeg"},
					"X-Foo":      []string{"bar"},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			res, err := NewTemplate(test.request())
			if err != nil {
				t.Fatal(err)
			}

			if !cmp.Equal(test.want, res) {
				t.Error(cmp.Diff(test.want, res))
			}
		})
	}
}
