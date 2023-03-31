package producer

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
)

func TestParseRange(t *testing.T) {
	tests := []struct {
		Input  string
		Result Range
	}{
		{
			"2",
			Range{First: 2, Last: 2},
		},
		{
			"1e1",
			Range{First: 10, Last: 10},
		},
		{
			"1-2",
			Range{First: 1, Last: 2},
		},
		{
			"5-800",
			Range{First: 5, Last: 800},
		},
		{
			"500-200",
			Range{First: 500, Last: 200},
		},
		{
			"-5-10",
			Range{First: -5, Last: 10},
		},
		{
			"-10--5",
			Range{First: -10, Last: -5},
		},
		{
			"1e1-1E2",
			Range{First: 10, Last: 100},
		},
		{
			"1-10e2",
			Range{First: 1, Last: 1000},
		},
		{
			"1-10e10",
			Range{First: 1, Last: 100000000000},
		},
		{
			"1e10-99999999999",
			Range{First: 10000000000, Last: 99999999999},
		},
		{
			"-10e2--10e1",
			Range{First: -1000, Last: -100},
		},
	}

	for _, test := range tests {
		t.Run(test.Input, func(t *testing.T) {
			r, err := NewRange(test.Input)
			if err != nil {
				t.Fatal(err)
			}

			if !cmp.Equal(test.Result, r) {
				t.Fatal(cmp.Diff(test.Result, r))
			}
		})
	}
}

func TestRange(t *testing.T) {
	tests := []struct {
		Inputs []string
		Values []string
	}{
		{
			[]string{"1-2"},
			[]string{"1", "2"},
		},
		{
			[]string{"5", "1-2"},
			[]string{"5", "1", "2"},
		},
		{
			[]string{"5-10", "20-23"},
			[]string{"5", "6", "7", "8", "9", "10", "20", "21", "22", "23"},
		},
		{
			[]string{"10-5"},
			[]string{"10", "9", "8", "7", "6", "5"},
		},
		{
			[]string{"10-5", "5-8"},
			[]string{"10", "9", "8", "7", "6", "5", "5", "6", "7", "8"},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			var ranges []Range

			for _, s := range test.Inputs {
				r, err := NewRange(s)
				if err != nil {
					t.Fatal(err)
				}

				ranges = append(ranges, r)
			}

			ch := make(chan string)
			count := make(chan int, 1)

			src := NewRanges(ranges, "%d")

			var eg errgroup.Group
			eg.Go(func() error {
				return src.Yield(context.Background(), ch, count)
			})

			var values []string
			eg.Go(func() error {
				for v := range ch {
					values = append(values, v)
				}

				return nil
			})

			err := eg.Wait()
			if err != nil {
				t.Fatal(err)
			}

			if !cmp.Equal(test.Values, values) {
				t.Fatal(cmp.Diff(test.Values, values))
			}

			c := <-count
			if c != len(test.Values) {
				t.Fatalf("count is wrong, want %d, got %d", len(test.Values), c)
			}
		})
	}
}
