package response

import (
	"bytes"
	"compress/gzip"
	"io"
	"math"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		input string
		stats TextStats
	}{
		{
			input: "",
			stats: TextStats{Bytes: 0, Words: 0, Lines: 0},
		},
		{
			input: "x",
			stats: TextStats{Bytes: 1, Words: 1, Lines: 0},
		},
		{
			input: "foo bar baz",
			stats: TextStats{Bytes: 11, Words: 3, Lines: 0},
		},
		{
			input: "foo   bar \t baz",
			stats: TextStats{Bytes: 15, Words: 3, Lines: 0},
		},
		{
			input: "foo   bar \r\n baz\n ",
			stats: TextStats{Bytes: 18, Words: 3, Lines: 2},
		},
		{
			input: "foo   <bar> xx2 </bar> \r\n baz\n ",
			stats: TextStats{Bytes: 31, Words: 5, Lines: 2},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			stats, err := Count(strings.NewReader(test.input))
			if err != nil {
				t.Fatal(err)
			}

			if stats != test.stats {
				t.Fatalf("wrong stats returned, want %v, got %v", test.stats, stats)
			}
		})
	}
}

func TestExtractBody(t *testing.T) {
	tests := []struct {
		body    string
		targets []*regexp.Regexp
		data    []string
	}{
		{
			body: "foo bar baz",
			targets: []*regexp.Regexp{
				regexp.MustCompile("foo"),
				regexp.MustCompile("foo.*baz"),
			},
			data: []string{
				"foo",
				"foo bar baz",
			},
		},
		{
			body: "foo bar baz",
			targets: []*regexp.Regexp{
				regexp.MustCompile("(...) baz"),
			},
			data: []string{
				"bar",
			},
		},
		{
			body: "foo bar baz",
			targets: []*regexp.Regexp{
				regexp.MustCompile("foo (...)(.*)"),
			},
			data: []string{
				"bar", " baz",
			},
		},
		{
			body: "foo bar baz",
			targets: []*regexp.Regexp{
				regexp.MustCompile(`(\S{3})`),
			},
			data: []string{
				"foo", "bar", "baz",
			},
		},
		{
			body: "foo bar baz",
			targets: []*regexp.Regexp{
				regexp.MustCompile(`\S{3}`),
			},
			data: []string{
				"foo", "bar", "baz",
			},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			var r Response
			err := r.ReadBody(&http.Response{Body: io.NopCloser(strings.NewReader(test.body))}, 1024*1024, false)
			if err != nil {
				t.Fatal(err)
			}

			r.ExtractBody(test.targets)

			if !reflect.DeepEqual(test.data, r.Extract) {
				t.Fatalf("wrong data, want %q, got %q", test.data, r.Extract)
			}
		})
	}
}

func TestDecompressBody(t *testing.T) {
	rawBody := []byte("body")
	compressedBody := compress(t, rawBody)

	tests := []struct {
		Algorithm            string
		Body                 []byte
		ExpectedResult       []byte
		DecompressionEnabled bool
		MaxBodySize          int
	}{
		{
			Algorithm:            "gzip",
			Body:                 compressedBody,
			ExpectedResult:       rawBody,
			DecompressionEnabled: true,
		},
		{
			Algorithm:            "gzip",
			Body:                 compressedBody,
			ExpectedResult:       compressedBody,
			DecompressionEnabled: false,
		},
		{
			Algorithm:            "",
			Body:                 compressedBody,
			ExpectedResult:       compressedBody,
			DecompressionEnabled: true,
		},
		{
			Algorithm:            "",
			Body:                 compressedBody,
			ExpectedResult:       compressedBody,
			DecompressionEnabled: false,
		},
		{
			Algorithm: "gzip",
			Body:      compress(t, bytes.Repeat(rawBody, 70)),
			ExpectedResult: []byte("bodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybo" +
				"dybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybod" +
				"ybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybodybody" +
				"bodybodybodybo"),
			DecompressionEnabled: true,
			MaxBodySize:          16,
		},
	}

	for i, test := range tests {
		test := test // capture variable

		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r := Response{
				HTTPResponse: &http.Response{
					Body:   io.NopCloser(bytes.NewReader(test.Body)),
					Header: make(http.Header),
				},
			}

			if test.Algorithm != "" {
				r.HTTPResponse.Header.Add("Content-Encoding", test.Algorithm)
			}

			if test.MaxBodySize == 0 {
				test.MaxBodySize = math.MaxInt
			}

			err := r.ReadBody(r.HTTPResponse, test.MaxBodySize, test.DecompressionEnabled)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			if !bytes.Equal(r.Body, test.ExpectedResult) {
				t.Fatalf("read body %q instead of %q", string(r.Body), string(test.ExpectedResult))
			}
		})
	}
}

func compress(tb testing.TB, data []byte) []byte {
	tb.Helper()

	buf := &bytes.Buffer{}

	gzw := gzip.NewWriter(buf)

	_, err := gzw.Write(data)
	if err != nil {
		tb.Fatalf("compress: %v", err)
	}

	err = gzw.Close()
	if err != nil {
		tb.Fatalf("close compressor: %v", err)
	}

	return buf.Bytes()
}
