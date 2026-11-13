package tesseract

import (
	"math"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v3/markup"
)

// parseTSV builds the lines and the mean confidence from the TSV output of tesseract.
// The lines hold the words of each block, paragraph and line, escaped as core markup.
func parseTSV(out string) (lines []string, confidence *float64) {
	var (
		order []string
		words = map[string][]string{}
		sum   float64
		n     int
	)
	rows := strings.Split(out, "\n")
	for _, row := range rows[1:] {
		cols := strings.Split(strings.TrimSuffix(row, "\r"), "\t")
		if len(cols) < 12 || cols[0] != "5" {
			continue
		}
		word := strings.TrimSpace(cols[11])
		if word == "" {
			continue
		}
		key := cols[2] + "/" + cols[3] + "/" + cols[4]
		if _, ok := words[key]; !ok {
			order = append(order, key)
		}
		words[key] = append(words[key], markup.EscapeText(word))
		sum += confidenceOf(cols[10])
		n++
	}
	if n == 0 {
		return nil, nil
	}
	for _, key := range order {
		lines = append(lines, strings.Join(words[key], " "))
	}
	mean := sum / float64(n)
	return lines, &mean
}

// confidenceOf turns a TSV confidence from 0 to 100 into a value from 0 to 1. Text that is not a number counts as 0.
func confidenceOf(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) {
		return 0
	}
	return min(100, max(0, f)) / 100
}
