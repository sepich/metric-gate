package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
)

// Data model for aggregation
type Series map[string]*Seria // string = MetricName
type Seria map[string]*SVal   // string = Labels.String()
type SVal struct {
	TimestampMs int64 // 0 = Now
	Value       float64
}

// Proxy handlers
type Proxy struct {
	Opts Options
}

func NewProxy(opts *Options) *Proxy {
	return &Proxy{Opts: *opts}
}

// index returns help message
func (p *Proxy) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<html><body>
	<h1>metric-gate</h1>
	<a href='/source'>/source</a> - original metrics from upstream<br/>
	<a href='/analyze'>/analyze</a> - analyze upstream response for metrics and label cardinality<br/>
	<a href='/metrics'>/metrics</a> - aggregated and filtered metrics from upstream<br/>
	<a href='/debug/pprof/'>/debug/pprof/</a> - pprof debug endpoints
	</body></html>`))
}

// src returns response from upstream as is
func (p *Proxy) src(w http.ResponseWriter, r *http.Request) {
	resp, err := p.get()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// analyze returns metrics and label cardinality from upstream response
func (p *Proxy) analyze(w http.ResponseWriter, r *http.Request) {
	resp, err := p.get()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	data, err := parse(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(getReport(data)))
}

// agg returns aggregated filtered metrics
func (p *Proxy) agg(w http.ResponseWriter, r *http.Request) {
	resp, err := p.get()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	data, err := p.parse(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	p.render(data, w)
}

func (p *Proxy) get() (*http.Response, error) {
	req, err := http.NewRequest("GET", p.Opts.Upstream, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

// parse unpacks and filters textformat
func (p *Proxy) parse(r io.Reader) (Series, error) {
	series := make(Series)
	scanner := bufio.NewScanner(r)
	lb := labels.NewBuilder(labels.EmptyLabels())
	for scanner.Scan() {
		line := scanner.Text()
		metricName, lbls, value, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		if metricName == "" {
			continue
		}

		// metric_relabel_configs
		lb.Reset(lbls)
		lb.Set("__name__", metricName)
		keep := relabel.ProcessBuilder(lb, p.Opts.Relabel...)
		if !keep {
			continue
		}
		lb.Del("__name__")
		ls := labelsString(lb.Labels())

		if series[metricName] == nil {
			tmp := make(Seria)
			tmp[ls] = &value
			series[metricName] = &tmp
		} else {
			tmp := *series[metricName]
			if tmp[ls] == nil {
				tmp[ls] = &value
			} else {
				tmp[ls].Value += value.Value
			}
		}
	}
	return series, nil
}

func (p *Proxy) render(series Series, w io.Writer) error {
	for metricName, seria := range series {
		for labels, value := range *seria {
			sb := strings.Builder{}
			sb.WriteString(metricName)
			if len(labels) > 2 {
				sb.WriteString(labels)
			}
			sb.WriteString(fmt.Sprintf(" %#v", value.Value))
			if value.TimestampMs > 0 {
				sb.WriteString(fmt.Sprintf(" %d", value.TimestampMs))
			}
			sb.WriteString("\n")
			w.Write([]byte(sb.String()))
		}
	}
	return nil
}
