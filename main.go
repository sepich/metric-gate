package main

import (
	"flag"
)

type Options struct {
	File string
}

func main() {
	var opts Options
	flag.StringVar(&opts.File, "file", "", "Analyze file for metrics and label cardinality")
	flag.Parse()

	if opts.File != "" {
		analyze(opts.File)
	}
}
