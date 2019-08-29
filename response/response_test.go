package response

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	var tests = []struct {
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
	var tests = []struct {
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
			err := r.ReadBody(strings.NewReader(test.body), 1024*1024)
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
