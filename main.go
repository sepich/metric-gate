package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/common/version"
)

type Options struct {
	File        string
	Upstream    string
	Labels      map[string]bool
	FilterNames string
	Port        int
}

// stringSliceFlag implements flag.Value
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var opts Options
	var ver bool
	var labels stringSliceFlag
	flag.StringVar(&opts.File, "file", "", "Analyze file for metrics and label cardinality and exit")
	flag.StringVar(&opts.Upstream, "upstream", "http://localhost:10254/metrics", "Source URL to get metrics from")
	flag.Var(&labels, "label", "Label to remove by aggregation, can be specified multiple times")
	flag.StringVar(&opts.FilterNames, "filter", "", "RegEx to drop metrics by name")
	flag.IntVar(&opts.Port, "port", 8080, "Port to serve aggregated metrics on")
	flag.BoolVar(&ver, "version", false, "Show version and exit")
	flag.Parse()

	if ver {
		fmt.Println(version.Print("prom-scrape-proxy"))
		os.Exit(0)
	}

	if !strings.Contains(opts.Upstream, "://") {
		opts.Upstream = "http://" + opts.Upstream
	}

	if opts.File != "" {
		fmt.Println(analyze(opts.File))
		os.Exit(0)
	}

	for _, label := range labels {
		opts.Labels[label] = true
	}

	proxy := NewProxy(&opts)
	http.HandleFunc("/", proxy.index)
	http.HandleFunc("/source", proxy.src)
	http.HandleFunc("/analyze", proxy.analyze)
	http.HandleFunc("/metrics", proxy.agg)
	fmt.Printf("Starting server on port %d\n", opts.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", opts.Port), nil); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
		os.Exit(1)
	}
}
