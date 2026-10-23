package phpkey

import (
	"math"
	"strconv"
)

// DigitsToInt reads a string of ASCII digits as the PHP cast (int) does. A value above the int64 range gives the
// int64 maximum.
func DigitsToInt(digits string) int64 {
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return math.MaxInt64
	}
	return n
}
