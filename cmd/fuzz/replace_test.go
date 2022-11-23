package fuzz

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseReplace(t *testing.T) {
	var tests = []struct {
		input   string
		replace Replace
		err     bool
	}{
		{
			input: "",
			err:   true,
		},
		{
			input: "zzz",
			err:   true,
		},
		{
			input: "file",
			err:   true,
		},
		{
			input: "file:",
			err:   true,
		},
		{
			input: "FUZZ:file:/tmp/foo.txt",
			replace: Replace{
				Name:    "FUZZ",
				Type:    "file",
				Options: "/tmp/foo.txt",
			},
		},
		{
			input: "AAA:xxfile:/tmp/foo.txt",
			replace: Replace{
				Name:    "AAA",
				Type:    "xxfile",
				Options: "/tmp/foo.txt",
			},
		},
		{
			input: "ZZ:range:1-100",
			replace: Replace{
				Name:    "ZZ",
				Type:    "range",
				Options: "1-100",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			r, err := ParseReplace(test.input)
			if test.err && err == nil {
				t.Fatal("want error, got nil")
			}

			if !test.err && err != nil {
				t.Fatal(err)
			}

			if !cmp.Equal(r, test.replace) {
				t.Fatal(cmp.Diff(test.replace, r))
			}
		})
	}
}
