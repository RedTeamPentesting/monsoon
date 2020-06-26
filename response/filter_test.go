package response

import "testing"

func TestFilterSize(t *testing.T) {
	var tests = []struct {
		spec   string
		size   int
		result bool
	}{
		{"200", 200, true},
		{"200", 123, false},
		{"200", 40000, false},
		{"-123", 0, true},
		{"-123", 200, false},
		{"-123", 123, true},
		{"123-", 0, false},
		{"123-", 200, true},
		{"123-", 123, true},
		{"123-222", 0, false},
		{"123-222", 122, false},
		{"123-222", 123, true},
		{"123-222", 124, true},
		{"123-222", 221, true},
		{"123-222", 222, true},
		{"123-222", 223, false},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			f, err := parseRangeFilterSpec(test.spec)
			if err != nil {
				t.Fatal(err)
			}

			result := f(test.size)
			if result != test.result {
				t.Fatalf("wrong result for %q testing size %d: want %v, got %v",
					test.spec, test.size, test.result, result)
			}
		})
	}
}
