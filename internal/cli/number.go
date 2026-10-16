package cli

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// formatNumber prints a number of a text report: at most 3 decimals without trailing zeros, "INF" for an
// infinity of either sign, and "-" for NaN, which stands for no value. It rounds the shortest decimal form
// half away from zero, as PHP number_format does, so 1.0005 prints 1.001.
func formatNumber(v float64) string {
	return formatDecimals(v, 3)
}

// formatDecimals prints v as formatNumber does, with at most decimals decimals.
func formatDecimals(v float64, decimals int) string {
	switch {
	case math.IsNaN(v):
		return "-"
	case math.IsInf(v, 0):
		return "INF"
	}
	negative, whole, fraction := roundDecimals(v, decimals)
	s := whole
	if fraction = strings.TrimRight(fraction, "0"); fraction != "" {
		s += "." + fraction
	}
	if negative && s != "0" {
		s = "-" + s
	}
	return s
}

// millis rounds seconds to whole milliseconds as formatNumber rounds them. v must be finite.
func millis(v float64) time.Duration {
	negative, whole, fraction := roundDecimals(v, 3)
	n, _ := strconv.ParseInt(whole+fraction, 10, 64)
	if negative {
		n = -n
	}
	return time.Duration(n) * time.Millisecond
}

// roundDecimals rounds the shortest decimal form of v half away from zero to decimals decimals.
func roundDecimals(v float64, decimals int) (negative bool, whole, fraction string) {
	s := strconv.FormatFloat(math.Abs(v), 'f', -1, 64)
	whole, fraction, _ = strings.Cut(s, ".")
	if len(fraction) <= decimals {
		return v < 0, whole, fraction + strings.Repeat("0", decimals-len(fraction))
	}
	roundUp := fraction[decimals] >= '5'
	digits := []byte(whole + fraction[:decimals])
	for i := len(digits) - 1; roundUp && i >= 0; i-- {
		if digits[i] == '9' {
			digits[i] = '0'
			continue
		}
		digits[i]++
		roundUp = false
	}
	if roundUp {
		digits = append([]byte{'1'}, digits...)
	}
	return v < 0, string(digits[:len(digits)-decimals]), string(digits[len(digits)-decimals:])
}
