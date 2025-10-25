package main

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"testing"

	"github.com/grafana/regexp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
)

func TestAgg(t *testing.T) {
	cases := []struct {
		input string
		want  string
		want2 string
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
			want2: m(
				`metric4 10 1751041454000`,
			),
		},
	}
	proxy := NewProxy(&Options{
		Relabel: map[string][]*relabel.Config{
			"metric_relabel_configs": {
				{
					Action: relabel.LabelDrop,
					Regex:  relabel.Regexp{Regexp: regexp.MustCompile("code")},
				},
				{
					Action:       relabel.Drop,
					SourceLabels: model.LabelNames{"__name__"},
					Regex:        relabel.Regexp{Regexp: regexp.MustCompile("metric4")},
				},
			},
			"sub": {
				{
					Action:       relabel.Keep,
					SourceLabels: model.LabelNames{"__name__"},
					Regex:        relabel.Regexp{Regexp: regexp.MustCompile("metric4")},
				},
			},
		},
	}, &slog.Logger{})
	for _, c := range cases {
		subsets := make(map[string]*Series)
		for s := range proxy.Opts.Relabel {
			subsets[s] = NewSeries()
		}
		err := proxy.parse(strings.NewReader(c.input), subsets)
		if err != nil {
			t.Errorf("parse(%s) error = %v", c.input, err)
		}

		// default subset
		var b strings.Builder
		render(subsets[default_subset], 0, &b)
		lines := strings.Split(strings.TrimSpace(b.String()), "\n")
		sort.Strings(lines) // sort result
		res := strings.Join(lines, "\n")
		if res != c.want {
			t.Errorf("got: '%s', want '%s'", res, c.want)
		}

		b.Reset()
		render(subsets["sub"], 0, &b)
		lines = strings.Split(strings.TrimSpace(b.String()), "\n")
		sort.Strings(lines) // sort result
		res = strings.Join(lines, "\n")
		if res != c.want2 {
			t.Errorf("got: '%s', want2 '%s'", res, c.want2)
		}
	}
}

func m(parts ...string) string {
	return strings.Join(parts, "\n")
}

func BenchmarkRender(b *testing.B) {
	s := NewSeries()
	for i := 0; i < 1000; i++ {
		tmp := make(Seria)
		tmp[fmt.Sprintf(`{canary="",controller_class="k8s.io/nginx",controller_namespace="ingress-nginx",controller_pod="ingress-nginx-controller-769b6d4b8c-kfh2r",ingress="",method="DELETE",namespace="",path="",service="",status="%d"}`, i)] = &SVal{Value: float64(i), TimestampMs: int64(i * 100)}
		s.data[fmt.Sprintf("nginx_ingress_controller_bytes_sent_bucket%d", i)] = &tmp
	}
	for b.Loop() {
		render(s, 0, &strings.Builder{})
	}
}
