package shell

import (
	"testing"
)

func TestJoin(t *testing.T) {
	var tests = []struct {
		args []string
		res  string
	}{
		{
			args: []string{"monsoon", "-r", "1-100", "http://localhost/FUZZ"},
			res:  "monsoon -r 1-100 http://localhost/FUZZ",
		},
		{
			args: []string{"monsoon", "--header", "x-foo: bar", "http://localhost/FUZZ"},
			res:  `monsoon --header "x-foo: bar" http://localhost/FUZZ`,
		},
		{
			args: []string{"monsoon", "--header", `x-foo: "bar"`, "http://localhost/FUZZ"},
			res:  `monsoon --header "x-foo: \"bar\"" http://localhost/FUZZ`,
		},
		{
			args: []string{"monsoon", "http://localhost/FUZZ?x1=1&x2=2"},
			res:  `monsoon "http://localhost/FUZZ?x1=1&x2=2"`,
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			res := Join(test.args)
			if res != test.res {
				t.Fatalf("wrong result, want\n  %s\ngot:\n  %s", test.res, res)
			}
		})
	}
}
