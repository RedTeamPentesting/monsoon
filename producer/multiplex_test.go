package producer

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
)

func mustParseRanges(rs string) *Ranges {
	var ranges []Range

	for _, s := range strings.Split(rs, ",") {
		r, err := NewRange(s)
		if err != nil {
			panic(err)
		}

		ranges = append(ranges, r)
	}

	return NewRanges(ranges, "%d")
}

func TestMultiplex(t *testing.T) {
	tests := []struct {
		names   []string
		sources []Source
		result  [][]string
	}{
		{
			names: []string{"FUZZ"},
			sources: []Source{
				mustParseRanges("1-3"),
			},
			result: [][]string{
				{"1"},
				{"2"},
				{"3"},
			},
		},
		{
			names: []string{"FUZZ"},
			sources: []Source{
				NewFile(strings.NewReader("1\n2\n3")),
			},
			result: [][]string{
				{"1"},
				{"2"},
				{"3"},
			},
		},
		{
			names: []string{"FUZZ", "FUZ2Z"},
			sources: []Source{
				mustParseRanges("1-3"),
				mustParseRanges("5-6"),
			},
			result: [][]string{
				{"1", "5"},
				{"1", "6"},
				{"2", "5"},
				{"2", "6"},
				{"3", "5"},
				{"3", "6"},
			},
		},
		{
			names: []string{"FUZZ", "FUZ2Z", "FUZ3Z"},
			sources: []Source{
				mustParseRanges("1-3"),
				mustParseRanges("5-6"),
				mustParseRanges("10"),
			},
			result: [][]string{
				{"1", "5", "10"},
				{"1", "6", "10"},
				{"2", "5", "10"},
				{"2", "6", "10"},
				{"3", "5", "10"},
				{"3", "6", "10"},
			},
		},
		{
			names: []string{"FUZZ", "FUZ2Z", "FUZ3Z"},
			sources: []Source{
				mustParseRanges("1-3"),
				mustParseRanges("5-6"),
				mustParseRanges("10-11"),
			},
			result: [][]string{
				{"1", "5", "10"},
				{"1", "5", "11"},
				{"1", "6", "10"},
				{"1", "6", "11"},
				{"2", "5", "10"},
				{"2", "5", "11"},
				{"2", "6", "10"},
				{"2", "6", "11"},
				{"3", "5", "10"},
				{"3", "5", "11"},
				{"3", "6", "10"},
				{"3", "6", "11"},
			},
		},
		{
			names: []string{"FUZZ", "FUZ2Z", "FUZ3Z"},
			sources: []Source{
				mustParseRanges("1-3"),
				NewFile(strings.NewReader("a\nb\n")),
				mustParseRanges("10-11"),
			},
			result: [][]string{
				{"1", "a", "10"},
				{"1", "a", "11"},
				{"1", "b", "10"},
				{"1", "b", "11"},
				{"2", "a", "10"},
				{"2", "a", "11"},
				{"2", "b", "10"},
				{"2", "b", "11"},
				{"3", "a", "10"},
				{"3", "a", "11"},
				{"3", "b", "10"},
				{"3", "b", "11"},
			},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			m := Multiplexer{
				Names:   test.names,
				Sources: test.sources,
			}

			var eg errgroup.Group

			ch := make(chan []string)
			count := make(chan int, 1)

			eg.Go(func() error {
				return m.Run(context.Background(), ch, count)
			})

			var values [][]string
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

			if !cmp.Equal(test.result, values) {
				t.Error(cmp.Diff(test.result, values))
			}

			c := <-count
			if c != len(test.result) {
				t.Errorf("wrong count, want %d, got %d", len(test.result), c)
			}
		})
	}
}
