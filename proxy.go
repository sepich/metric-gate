package main

import (
	"io"
	"net/http"
)

type Proxy struct {
	opts Options
}

func NewProxy(opts *Options) *Proxy {
	return &Proxy{opts: *opts}
}

// index returns help message
func (p *Proxy) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<html><body>
	<h1>prom-scrape-proxy</h1>
	<a href='/source'>/source</a> - original metrics from upstream<br/>
	<a href='/analyze'>/analyze</a> - analyze upstream response for metrics and label cardinality<br/>
	<a href='/metrics'>/metrics</a> - aggregated and filtered metrics from upstream
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

	data, err := parseIO(resp.Body)
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
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("todo"))
}

func (p *Proxy) get() (*http.Response, error) {
	req, err := http.NewRequest("GET", p.opts.Upstream, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}
