package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Metric struct {
	Labels map[string]map[string]bool
	Count  int
}

func (m *Metric) AddLabel(key, value string) {
	if m.Labels[key] == nil {
		m.Labels[key] = make(map[string]bool)
	}
	m.Labels[key][value] = true
}

func analyze(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Sprintf("Error opening file: %s", err)
	}
	defer file.Close()

	data, err := parse(file)
	if err != nil {
		return fmt.Sprintf("Error parsing file: %s", err)
	}
	return getReport(data)
}

func parse(file io.Reader) (map[string]*Metric, error) {
	scanner := bufio.NewScanner(file)
	series := make(map[string]*Metric)
	for scanner.Scan() {
		line := scanner.Text()
		metricName, labels, _, err := parseLine(line)
		if err != nil {
			return nil, err
		}
		if metricName == "" {
			continue
		}

		metric, ok := series[metricName]
		if !ok {
			metric = &Metric{
				Labels: make(map[string]map[string]bool),
			}
			series[metricName] = metric
		}
		for key, value := range labels {
			metric.AddLabel(key, value)
		}
		metric.Count++
	}
	return series, nil
}

func getReport(data map[string]*Metric) (res string) {
	sortedMetrics := make([]string, 0, len(data))
	for k := range data {
		sortedMetrics = append(sortedMetrics, k)
	}
	sort.Slice(sortedMetrics, func(i, j int) bool {
		return data[sortedMetrics[i]].Count > data[sortedMetrics[j]].Count
	})
	for _, m := range sortedMetrics {
		n := data[m].Count
		if n == 0 {
			n = 1 // metric without labels
		}
		res += fmt.Sprintf("\n%d %s\n", n, m)
		s := []string{}
		for l, v := range data[m].Labels {
			if len(v) <= 5 {
				labels := make([]string, 0, len(v))
				for k := range v {
					labels = append(labels, k)
				}
				s = append(s, fmt.Sprintf("  %d %s [%s]\n", len(v), l, strings.Join(labels, ", ")))
			} else {
				s = append(s, fmt.Sprintf("  %d %s\n", len(v), l))
			}
		}
		sort.Slice(s, func(i, j int) bool {
			x, _ := strconv.ParseInt(strings.Split(strings.TrimSpace(s[i]), " ")[0], 10, 64)
			y, _ := strconv.ParseInt(strings.Split(strings.TrimSpace(s[j]), " ")[0], 10, 64)
			return x > y
		})
		res += strings.Join(s, "")
	}
	return res
}
