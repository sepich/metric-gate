package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	_ "net/http/pprof"

	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

type Options struct {
	File     string
	Upstream string
	Relabel  []*relabel.Config
	Port     int
}

func main() {
	var opts Options
	pflag.StringVarP(&opts.File, "file", "f", "", "Analyze file for metrics and label cardinality and exit")
	pflag.StringVarP(&opts.Upstream, "upstream", "H", "http://localhost:10254/metrics", "Source URL to get metrics from")
	var re = pflag.StringP("relabel", "", "", "metric_relabel_configs contents")
	var reFile = pflag.StringP("relabel-file", "", "", "metric_relabel_configs file path")
	pflag.IntVarP(&opts.Port, "port", "p", 8080, "Port to serve aggregated metrics on")
	var ver = pflag.BoolP("version", "v", false, "Show version and exit")
	pflag.Parse()

	if *ver {
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
	if *re != "" && *reFile != "" {
		fmt.Println("Error: both `relabel` and `relabel-file` specified")
		os.Exit(1)
	}
	if *reFile != "" {
		data, err := os.ReadFile(*reFile)
		if err != nil {
			fmt.Println("Error reading relabel-file:", err)
			os.Exit(1)
		}
		err = yaml.Unmarshal(data, &opts.Relabel)
		if err != nil {
			fmt.Println("Error parsing relabel-file:", err)
			os.Exit(1)
		}
	}
	if *re != "" {
		err := yaml.Unmarshal([]byte(*re), &opts.Relabel)
		if err != nil {
			fmt.Println("Error parsing relabel:", err)
			os.Exit(1)
		}
	}

	proxy := NewProxy(&opts)
	http.HandleFunc("/", proxy.index)
	http.HandleFunc("/source", proxy.src)
	http.HandleFunc("/analyze", proxy.analyze)
	http.HandleFunc("/metrics", proxy.agg)
	fmt.Printf("Starting server, port=%d, version=%s\n", opts.Port, version.Version)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", opts.Port), nil); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
		os.Exit(1)
	}
}
