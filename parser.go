package main

import (
	"bytes"
	"fmt"
	"strconv"

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
	i := 0
	for ; i < len(line) && (line[i] == ' ' || line[i] == '\t'); i++ { // not needed
	}
	if len(line) == 0 || line[i] == '#' {
		return "", nil, SVal{}, nil
	}

	lb := labels.NewBuilder(labels.EmptyLabels())
	j := i
	for j < len(line) {
		if line[j] == ' ' || line[j] == '{' {
			name = line[i:j]
			for ; j < len(line) && line[j] == ' '; j++ {
			}
			if line[j] != '{' {
				i = j
				break
			}
			// labels
			i = j + 1
			lname, lvalue := "", ""
			for {
				for ; i < len(line) && line[i] == ' '; i++ {
				}
				if line[i] == '}' {
					i++
					for ; i < len(line) && line[i] == ' '; i++ {
					}
					break
				}
				// labelname
				j = i + 1
				for ; j < len(line); j++ {
					if line[j] == '}' {
						return "", nil, SVal{}, fmt.Errorf("invalid labelName}: %s", line)
					}
					if line[j] == ' ' || line[j] == '=' {
						lname = line[i:j]
						for ; j < len(line) && line[j] == ' '; j++ {
						}
						if line[j] == '=' {
							i = j + 1
							break
						}
						return "", nil, SVal{}, fmt.Errorf("invalid labelName=: %s", line)
					}
				}
				for ; i < len(line) && line[i] == ' '; i++ {
				}

				// labelvalue
				if line[i] != '"' {
					return "", nil, SVal{}, fmt.Errorf("invalid labelValue: %s", line)
				}
				i++
				j = i
				for ; j < len(line); j++ {
					if line[j] == '"' && line[j-1] != '\\' {
						lvalue = line[i:j]
						i = j + 1
						break
					}
				}
				if lname == "__name__" {
					name = lvalue
				} else {
					lb.Set(lname, lvalue)
				}

				for ; i < len(line) && line[i] == ' '; i++ {
				}
				if line[i] == ',' {
					i++
				}
			}
			break
		}
		j++
	}

	// value
	j = i + 1
	for ; j < len(line) && line[j] != ' '; j++ {
	}
	if i >= len(line) {
		return "", nil, SVal{}, fmt.Errorf("invalid line, no value: %s", line)
	}
	value.Value, err = strconv.ParseFloat(line[i:j], 64)
	if err != nil {
		return "", nil, SVal{}, fmt.Errorf("invalid value %s: %s", line[i:j], line)
	}
	// ts
	i = j + 1
	for ; i < len(line) && line[i] == ' '; i++ {
	}
	j = i + 1
	for ; j < len(line) && line[j] != ' '; j++ {
	}
	if i < len(line) {
		value.TimestampMs, err = strconv.ParseInt(line[i:j], 10, 64)
		if err != nil {
			return "", nil, SVal{}, fmt.Errorf("invalid timestamp: %s", line)
		}
	}

	return name, lb.Labels(), value, nil
}
