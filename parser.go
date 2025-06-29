package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
)

// more compact output for Labels.String()
func labelsString(ls labels.Labels) string {
	var bytea [1024]byte // On stack to avoid memory allocation while building the output.
	b := bytes.NewBuffer(bytea[:0])

	b.WriteByte('{')
	first := true
	for _, l := range ls {
		if l.Value == "" {
			continue
		}
		if !first {
			b.WriteByte(',')
		}
		b.WriteString(l.Name)
		b.WriteByte('=')
		b.WriteByte('"')
		b.WriteString(l.Value)
		b.WriteByte('"')
		first = false
	}
	b.WriteByte('}')
	return b.String()
}

// parseLine is a simplified expfmt.TextToMetricFamilies to unpack textformat, returns empty metricName if line is a comment or blank
// https://prometheus.io/docs/instrumenting/exposition_formats/
func parseLine(line string) (name string, lbls labels.Labels, value SVal, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", nil, SVal{}, nil
	}

	lb := labels.NewBuilder(labels.EmptyLabels())
	sVals := ""
	sLabels := ""
	i := strings.LastIndex(line, "}")
	if i == -1 {
		name, sVals, _ = strings.Cut(line, " ")
	} else {
		tmp := strings.SplitN(line, "{", 2)
		if len(tmp) != 2 {
			return "", nil, SVal{}, fmt.Errorf("invalid line: %s", line)
		}
		name = strings.TrimSpace(tmp[0])
		sVals = strings.TrimSpace(tmp[1][i-len(tmp[0]):])
		sLabels = strings.TrimSpace(tmp[1][:i-len(tmp[0])-1])

		for {
			if len(sLabels) == 0 {
				break
			}
			// label name
			i := strings.Index(sLabels, "=")
			if i == -1 || i >= len(sLabels)-2 {
				return "", nil, SVal{}, fmt.Errorf("invalid labelName: %s", line)
			}
			lName := strings.TrimSpace(sLabels[:i])

			// quoted value
			sLabels = strings.TrimSpace(sLabels[i+1:])
			if sLabels[0] != '"' {
				return "", nil, SVal{}, fmt.Errorf("invalid labelValue: %s", line)
			}
			found := false
			for j := 1; j < len(sLabels); j++ {
				if sLabels[j] == '"' && sLabels[j-1] != '\\' {
					i = j
					found = true
					break
				}
			}
			if !found {
				return "", nil, SVal{}, fmt.Errorf("invalid labelValue: %s", line)
			}
			if lName == "__name__" {
				name = sLabels[1:i]
			} else {
				lb.Set(lName, sLabels[1:i])
			}

			// trailing comma
			if i == len(sLabels)-1 {
				break
			}
			sLabels = strings.TrimSpace(sLabels[i+1:])
			if len(sLabels) > 0 && sLabels[0] != ',' {
				return "", nil, SVal{}, fmt.Errorf("invalid labelDelim: %s", line)
			}
			sLabels = strings.TrimSpace(sLabels[1:])
		}
	}

	// value and timestamp
	found := false
	for _, tmp := range strings.Split(strings.TrimSpace(sVals), " ") {
		if tmp == "" {
			continue
		}
		if !found {
			value.Value, err = strconv.ParseFloat(tmp, 64)
			if err != nil {
				return "", nil, SVal{}, fmt.Errorf("invalid value: %s", line)
			}
			found = true
			continue
		}
		value.TimestampMs, err = strconv.ParseInt(tmp, 10, 64)
		if err != nil {
			return "", nil, SVal{}, fmt.Errorf("invalid TS: %s", line)
		}
		break // stop reading after timestamp
	}

	return name, lb.Labels(), value, nil
}
