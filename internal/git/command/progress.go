package command

import (
	"regexp"
	"strconv"
	"strings"
)

type Progress struct {
	Stage    string
	Percent  int
	Fraction float64
}
type ProgressAnalyzer struct {
	fractions map[string]float64
	values    map[string]float64
}

var progressPatterns = []struct {
	stage   string
	pattern *regexp.Regexp
	weight  float64
}{
	{"counting", regexp.MustCompile(`(?i).*Counting objects: +(\d+).*`), .05},
	{"compressing", regexp.MustCompile(`(?i).*Compressing objects: +(\d{1,3})%.*`), .1},
	{"receiving", regexp.MustCompile(`(?i).*(?:Receiving|Writing) objects: +(\d{1,3})%.*`), .8},
	{"resolving", regexp.MustCompile(`(?i).*Resolving deltas: +(\d{1,3})%.*`), .05},
}

func NewProgressAnalyzer() *ProgressAnalyzer {
	return &ProgressAnalyzer{fractions: map[string]float64{}, values: map[string]float64{}}
}

func (a *ProgressAnalyzer) Analyze(line string) (Progress, bool) {
	for i, op := range progressPatterns {
		m := op.pattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return Progress{}, false
		}
		value := float64(n) / 100
		if i == 0 {
			value = float64(n) / 5000
			if n > 5000 {
				value = 1
			}
		}
		a.values[op.stage] = value
		for j := 0; j < i; j++ {
			a.values[progressPatterns[j].stage] = 1
		}
		total := 0.0
		for _, stage := range progressPatterns {
			if v, ok := a.values[stage.stage]; ok {
				total += stage.weight * v
			}
		}
		if total < 0 {
			total = 0
		}
		if total > 1 {
			total = 1
		}
		percent := n
		if i == 0 {
			percent = int(value * 100)
		}
		return Progress{Stage: op.stage, Percent: percent, Fraction: total}, true
	}
	return Progress{}, false
}

func ParseProgress(line string) (Progress, bool) {
	return NewProgressAnalyzer().Analyze(strings.TrimSpace(line))
}
