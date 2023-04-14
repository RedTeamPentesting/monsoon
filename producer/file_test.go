package producer

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
)

func TestFile(t *testing.T) {
	tests := []struct {
		Input  string
		Values []string
	}{
		{
			"foo",
			[]string{"foo"},
		},
		{
			"foo\n",
			[]string{"foo"},
		},
		{
			"foo\nbar",
			[]string{"foo", "bar"},
		},
		{
			"foo\nbar\n",
			[]string{"foo", "bar"},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			src := NewFile(strings.NewReader(test.Input), true)

			// test the source twice, so we check if it really yields the same
			// values
			for i := 0; i < 2; i++ {
				ch := make(chan string)
				count := make(chan int, 1)

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
					t.Fatalf("run %d: %v", i, err)
				}

				if !cmp.Equal(test.Values, values) {
					t.Fatalf("run %d: %v", i, cmp.Diff(test.Values, values))
				}
			}
		})
	}
}
