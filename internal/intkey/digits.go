package intkey

import (
	"math"
	"strconv"
)

// DigitsToInt reads a string of ASCII digits as an integer. A value above the int64 range gives the
// int64 maximum.
func DigitsToInt(digits string) int64 {
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return math.MaxInt64
	}
	return n
}
