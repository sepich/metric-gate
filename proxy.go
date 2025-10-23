package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
)

// Data model for aggregation
type Series struct {
	data map[string]*Seria // string = MetricName
	mu   sync.Mutex
}
type Seria map[string]*SVal // string = Labels.String()
type SVal struct {
	TimestampMs int64 // 0 = Now
	Value       float64
}

func NewSeries() *Series {
	return &Series{
		data: make(map[string]*Seria),
	}
}
func (s *Series) Add(metricName string, ls string, value SVal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data[metricName] == nil {
		tmp := make(Seria)
		tmp[ls] = &value
		s.data[metricName] = &tmp
	} else {
		tmp := *s.data[metricName]
		if tmp[ls] == nil {
			tmp[ls] = &value
		} else {
			tmp[ls].Value += value.Value
		}
	}
}

// Proxy handlers
type Proxy struct {
	Opts   Options
	logger *slog.Logger
}

func NewProxy(opts *Options, logger *slog.Logger) *Proxy {
	return &Proxy{Opts: *opts, logger: logger}
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

// src returns the response from upstream as is
func (p *Proxy) src(w http.ResponseWriter, r *http.Request) {
	resp, err := get(p.Opts.Upstream)
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
	resp, err := get(p.Opts.Upstream)
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
	series := NewSeries()
	hosts := []string{p.Opts.Upstream}

	if p.Opts.Resolve != nil {
		ips, err := net.LookupIP(p.Opts.Resolve.Hostname())
		if err != nil {
			p.logger.Error("Error resolving upstream", "host", p.Opts.Resolve.Host, "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		hosts = hosts[:0]
		for _, ip := range ips {
			hosts = append(hosts, ip.String())
		}
		p.logger.Debug("Resolved upstream", "hosts", hosts)
	}

	wg := sync.WaitGroup{}
	wg.Add(len(hosts))
	for _, h := range hosts {
		go func(host string) {
			defer wg.Done()
			u := host
			if p.Opts.Resolve != nil {
				t := url.URL{
					Scheme:   p.Opts.Resolve.Scheme,
					Host:     host + ":" + p.Opts.Resolve.Port(),
					Path:     p.Opts.Resolve.Path,
					RawQuery: p.Opts.Resolve.RawQuery,
				}
				u = t.String()
			}
			ctx, cancel := context.WithTimeout(r.Context(), p.Opts.Timeout)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
			if err != nil {
				p.logger.Error("Error creating request", "host", host, "err", err)
				return
			}
			if p.Opts.Resolve != nil {
				req.Host = p.Opts.Resolve.Hostname() // preserve the original Host header
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				p.logger.Error("Request failed", "host", host, "err", err)
				return
			}
			defer resp.Body.Close()

			err = p.parse(resp.Body, series)
			if err != nil {
				p.logger.Error("Error parsing response", "host", host, "err", err)
				return
			}
		}(h)
	}
	wg.Wait()

	// only fail if all wg failed?
	if len(series.data) == 0 {
		http.Error(w, "Error getting any metrics from upstream", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	render(series, w)
}

func get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

// parse unpacks and filters textformat
func (p *Proxy) parse(r io.Reader, series *Series) error {
	scanner := bufio.NewScanner(r)
	lb := labels.NewBuilder(labels.EmptyLabels())
	for scanner.Scan() {
		line := scanner.Text()
		metricName, lbls, value, err := parseLine(line)
		if err != nil {
			return err
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
		series.Add(metricName, ls, value)
	}
	return nil
}

func render(series *Series, w io.Writer) {
	for metricName, seria := range series.data {
		for labels, value := range *seria {
			w.Write([]byte(metricName))
			if len(labels) > 2 {
				w.Write([]byte(labels))
			}
			w.Write([]byte(fmt.Sprintf(" %#v", value.Value)))
			if value.TimestampMs > 0 {
				w.Write([]byte(fmt.Sprintf(" %d", value.TimestampMs)))
			}
			w.Write([]byte("\n"))
		}
	}
}
