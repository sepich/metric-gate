package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

const default_subset = "metric_relabel_configs"

type Options struct {
	File     string
	Upstream string
	Relabel  map[string][]*relabel.Config
	Port     int
	Timeout  time.Duration
	Resolve  *url.URL
}

func main() {
	var opts Options
	pflag.StringVarP(&opts.File, "file", "f", "", "Analyze file for metrics and label cardinality and exit")
	pflag.StringVarP(&opts.Upstream, "upstream", "H", "http://localhost:10254/metrics", "Source URL to get metrics from. The scheme may be prefixed with 'dns+' to resolve and aggregate multiple targets")
	var re = pflag.StringP("relabel", "", "", "Contents of yaml file with metric_relabel_configs")
	var reFile = pflag.StringP("relabel-file", "", "", "Path to yaml file with metric_relabel_configs (mutually exclusive)")
	pflag.DurationVarP(&opts.Timeout, "scrape-timeout", "t", 15*time.Second, "Timeout for upstream requests")
	pflag.IntVarP(&opts.Port, "port", "p", 8080, "Port to serve aggregated metrics on")
	var ver = pflag.BoolP("version", "v", false, "Show version and exit")
	var logLevel = pflag.StringP("log-level", "", "info", "Log level (info, debug)")
	pflag.Parse()
	logger := getLogger(*logLevel)

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
		logger.Error("Error: both `relabel` and `relabel-file` specified")
		os.Exit(1)
	}
	if *reFile != "" {
		data, err := os.ReadFile(*reFile)
		if err != nil {
			logger.Error("Error reading relabel-file", "err", err)
			os.Exit(1)
		}
		err = yaml.Unmarshal(data, &opts.Relabel)
		if err != nil {
			logger.Error("Error parsing relabel-file", "err", err)
			os.Exit(1)
		}
	}
	if *re != "" {
		err := yaml.Unmarshal([]byte(*re), &opts.Relabel)
		if err != nil {
			logger.Error("Error parsing relabel", "err", err)
			os.Exit(1)
		}
	}
	for s := range opts.Relabel {
		for _, r := range opts.Relabel[s] {
			if err := r.Validate(); err != nil {
				logger.Error("Error validating relabel config", "section", s, "err", err)
				os.Exit(1)
			}
		}
	}
	if len(opts.Relabel) > 0 && opts.Relabel[default_subset] == nil {
		logger.Error("Error: relabel config key `metric_relabel_configs` is not defined")
		os.Exit(1)
	}
	if len(opts.Relabel) == 0 {
		opts.Relabel = map[string][]*relabel.Config{
			default_subset: {},
		}
	}

	if strings.HasPrefix(opts.Upstream, "dns+") {
		opts.Upstream = opts.Upstream[4:]
		parts, err := url.Parse(opts.Upstream)
		if err != nil {
			logger.Error("Error parsing upstream as url", "err", err)
			os.Exit(1)
		}
		opts.Resolve = parts
	}

	proxy := NewProxy(&opts, logger)
	http.HandleFunc("/", proxy.index)
	http.HandleFunc("/source", proxy.src)
	http.HandleFunc("/analyze", proxy.analyze)
	http.HandleFunc("/metrics", proxy.agg)
	http.HandleFunc("/metrics/", proxy.agg)
	http.HandleFunc("/metrics/{subset}", proxy.agg)
	logger.Info("Starting server", "port", opts.Port, "version", version.Version)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", opts.Port), nil); err != nil {
		logger.Error("Error starting server", "err", err)
		os.Exit(1)
	}
}

func getLogger(logLevel string) *slog.Logger {
	var l = slog.LevelInfo
	if logLevel == "debug" {
		l = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     l,
		AddSource: logLevel == "debug",
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			if a.Key == slog.SourceKey {
				s := a.Value.String()
				i := strings.LastIndex(s, "/")
				j := strings.LastIndex(s, " ")
				a.Value = slog.StringValue(s[i+1:j] + ":" + s[j+1:len(s)-1])
			}
			if a.Key == slog.LevelKey {
				a.Value = slog.StringValue(strings.ToLower(a.Value.String()))
			}
			return a
		},
	}))
}
