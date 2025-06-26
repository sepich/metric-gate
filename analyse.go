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

	data, err := parseIO(file)
	if err != nil {
		return fmt.Sprintf("Error parsing file: %s", err)
	}
	return getReport(data)
}

func parseIO(file io.Reader) (map[string]*Metric, error) {
	scanner := bufio.NewScanner(file)
	series := make(map[string]*Metric)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid metric format:\n%s", line)
		}
		metricName := parts[0]
		labels := make(map[string]string)
		if strings.Contains(metricName, "{") {
			nameAndLabels := strings.SplitN(metricName, "{", 2)
			metricName = nameAndLabels[0]
			labelStr := strings.TrimSuffix(nameAndLabels[1], "}")
			for _, label := range strings.Split(labelStr, ",") {
				labelParts := strings.SplitN(label, "=", 2)
				if len(labelParts) != 2 {
					continue
				}
				key := labelParts[0]
				val := strings.Trim(labelParts[1], `"`)
				labels[key] = val
			}
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
