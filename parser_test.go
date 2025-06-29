package main

import (
	"testing"
)

func TestParseLine(t *testing.T) {
	cases := []struct {
		line   string
		name   string
		labels string
		value  SVal
		err    error
	}{
		{
			line:   `metric1 10`,
			name:   "metric1",
			labels: "{}",
			value:  SVal{Value: 10, TimestampMs: 0},
			err:    nil,
		},
		{
			line:   `metric2 10 1751041454000`,
			name:   "metric2",
			labels: "{}",
			value:  SVal{Value: 10, TimestampMs: 1751041454000},
			err:    nil,
		},
		{
			line:   ` metric3  10 `,
			name:   "metric3",
			labels: "{}",
			value:  SVal{Value: 10, TimestampMs: 0},
			err:    nil,
		},
		{
			line:   `metric4{code="200", method="GET"} 10`,
			name:   "metric4",
			labels: `{code="200",method="GET"}`,
			value:  SVal{Value: 10, TimestampMs: 0},
			err:    nil,
		},
		{
			line:   `metric5 { method = "GET", code = "200" } 10`,
			name:   "metric5",
			labels: `{code="200",method="GET"}`,
			value:  SVal{Value: 10, TimestampMs: 0},
			err:    nil,
		},
		{
			line:   `metric6{code="200", method="GET",}  10  1751041454000 # exemplar`,
			name:   "metric6",
			labels: `{code="200",method="GET"}`,
			value:  SVal{Value: 10, TimestampMs: 1751041454000},
			err:    nil,
		}, {
			line:   `metric7{b="b", a="a,b,c" ,d="a=b", C="{\nA\"B"} 10`,
			name:   "metric7",
			labels: `{C="{\nA\"B",a="a,b,c",b="b",d="a=b"}`,
			value:  SVal{Value: 10, TimestampMs: 0},
			err:    nil,
		},
		{
			line:   `{method="GET",code="200",__name__="metric8", foo=""} 10`,
			name:   "metric8",
			labels: `{code="200",method="GET"}`,
			value:  SVal{Value: 10, TimestampMs: 0},
			err:    nil,
		},
		{
			line:   `metric9{} 10`,
			name:   "metric9",
			labels: "{}",
			value:  SVal{Value: 10, TimestampMs: 0},
			err:    nil,
		},
	}

	for _, c := range cases {
		name, labels, value, err := parseLine(c.line)
		if name != c.name {
			t.Errorf("expected name %s, got %s", c.name, name)
		}
		if labelsString(labels) != c.labels {
			t.Errorf("expected labels %v, got %v", c.labels, labels)
		}
		if value != c.value {
			t.Errorf("expected value %v, got %v", c.value, value)
		}
		if err != c.err {
			t.Errorf("expected error %v, got %v", c.err, err)
		}
	}
}
