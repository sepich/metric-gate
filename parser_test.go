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
			line:   ``,
			name:   "",
			labels: "{}",
			value:  SVal{},
			err:    nil,
		},
		{
			line:   `# helptext`,
			name:   "",
			labels: "{}",
			value:  SVal{},
			err:    nil,
		},
		{
			line:   ` 	# helptext`,
			name:   "",
			labels: "{}",
			value:  SVal{},
			err:    nil,
		},
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
			line:   `metric3  10 `,
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
		},
		{
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
		if err != c.err {
			t.Errorf("expected error %v, got %v", c.err, err)
		}
		if name != c.name {
			t.Errorf("expected name %s, got %s", c.name, name)
		}
		if labelsString(labels) != c.labels {
			t.Errorf("(%s) expected labels %v, got %v", c.name, c.labels, labelsString(labels))
		}
		if value != c.value {
			t.Errorf("expected value %v, got %v", c.value, value)
		}

	}

	// test cases that malformed input does not cause panic
	errorCases := []struct {
		name string
		line string
	}{
		{
			name: "truncated label value (missing closing quote)",
			line: `nginx_ingress_controller_orphan_ingress{controller_class="k8s.io/nginx",controller_namespace="ingress-nginx",controller_pod="ingress-nginx-controller-webhook-8c4fd48d6-7bwqc",ingress="cm-acme-http-solver-chgl4`,
		},
		{
			name: "line ending after equals sign",
			line: `metric{label=`,
		},
		{
			name: "line ending with spaces after equals",
			line: `metric{label=   `,
		},
		{
			name: "incomplete label section",
			line: `metric{label`,
		},
		{
			name: "missing label value quote",
			line: `metric{label=value} 10`,
		},
	}

	for _, c := range errorCases {
		_, _, _, err := parseLine(c.line)
		if err == nil {
			t.Errorf("(%s) expected error for malformed input: %s", c.name, c.line)
		}
	}
}

func BenchmarkParseLine(b *testing.B) {
	s := `nginx_ingress_controller_request_duration_seconds_sum{canary="",controller_class="k8s.io/nginx-test",controller_namespace="ingress-nginx",controller_pod="ingress-nginx-controller-test-769b6d4b8c-kfh2r",ingress="helm-testing-t-7a97764ipl-test-services-helm-essential",method="GET",namespace="testing-t-7a97764ipl",path="/actuator/health",service="helm-testing-t-7a97764ipl-test-services-helm",status="2xx"} 151.3409999999997`
	for b.Loop() {
		parseLine(s)
	}
}
