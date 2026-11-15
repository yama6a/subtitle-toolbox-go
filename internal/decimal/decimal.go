// Package decimal reads strings of ASCII decimal digits.
package decimal

import (
	"math"
	"strconv"
	"strings"
)

// IsCanonical reports whether s is a canonical decimal int64: no plus sign, no leading zero, no -0. A format data
// key of this form reads as a list index.
func IsCanonical(s string) bool {
	digits := strings.TrimPrefix(s, "-")
	if digits == "" || (digits[0] == '0' && s != "0") {
		return false
	}
	for i := range len(digits) {
		if digits[i] < '0' || digits[i] > '9' {
			return false
		}
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

// Saturated reads a string of ASCII digits as an integer. A value above the int64 range gives the
// int64 maximum.
func Saturated(digits string) int64 {
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return math.MaxInt64
	}
	return n
}
