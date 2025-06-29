package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	_ "net/http/pprof"

	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/model/relabel"
	"gopkg.in/yaml.v2"
)

type Options struct {
	File     string
	Upstream string
	Relabel  []*relabel.Config
	Port     int
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
	var re, reFile string
	flag.StringVar(&opts.File, "file", "", "Analyze file for metrics and label cardinality and exit")
	flag.StringVar(&opts.Upstream, "upstream", "http://localhost:10254/metrics", "Source URL to get metrics from")
	flag.StringVar(&re, "relabel", "", "metric_relabel_configs contents")
	flag.StringVar(&reFile, "relabel-file", "", "metric_relabel_configs file path")
	flag.IntVar(&opts.Port, "port", 8080, "Port to serve aggregated metrics on")
	flag.BoolVar(&ver, "version", false, "Show version and exit")
	flag.Parse()

	if ver {
		fmt.Println(version.Print("metric-gate"))
		os.Exit(0)
	}

	if opts.File != "" {
		fmt.Println(analyze(opts.File))
		os.Exit(0)
	}

	if !strings.Contains(opts.Upstream, "://") {
		opts.Upstream = "http://" + opts.Upstream
	}
	if re != "" && reFile != "" {
		fmt.Println("Error: both `relabel` and `relabel-file` specified")
		os.Exit(1)
	}
	if reFile != "" {
		data, err := os.ReadFile(reFile)
		if err != nil {
			fmt.Println("Error reading relabel-file:", err)
			os.Exit(1)
		}
		yaml.Unmarshal(data, &opts.Relabel)
	}
	if re != "" {
		yaml.Unmarshal([]byte(re), &opts.Relabel)
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
