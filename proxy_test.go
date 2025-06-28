package main

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestAgg(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: `metric1 10`,
			want:  `metric1 10`,
		},
		{
			input: m(
				`metric2 10`,
				`metric1 10 1751041454000`,
			),
			want: m(
				`metric1 10 1751041454000`,
				`metric2 10`,
			),
		},
		{
			input: m(
				`metric3{code="200", method="POST"} 1`,
				`metric3{code="200", method="GET"} 1`,
				`metric4 10 1751041454000`,
				`metric3{code="200", method="GET"} 2`,
			),
			want: m(
				`metric3{method="GET"} 3`,
				`metric3{method="POST"} 1`,
			),
		},
	}
	proxy := NewProxy(&Options{
		Labels: map[string]bool{
			"code": true,
		},
		FilterNames: regexp.MustCompile("metric4"),
	})
	for _, c := range cases {
		data, err := proxy.parse(strings.NewReader(c.input))
		if err != nil {
			t.Errorf("parse(%s) error = %v", c.input, err)
		}
		var b strings.Builder
		proxy.render(data, &b)
		// sort lines
		lines := strings.Split(strings.TrimSpace(b.String()), "\n")
		sort.Strings(lines)
		res := strings.Join(lines, "\n")
		if res != c.want {
			t.Errorf("got: '%s', want '%s'", res, c.want)
		}
	}
}

func m(parts ...string) string {
	return strings.Join(parts, "\n")
}
