// Package phpkey holds the rules by which PHP converts array keys.
package phpkey

import (
	"strconv"
	"strings"
)

// IsIntKey reports whether PHP turns the array key into an integer: a canonical decimal int64.
func IsIntKey(key string) bool {
	digits := strings.TrimPrefix(key, "-")
	if digits == "" || (digits[0] == '0' && key != "0") {
		return false
	}
	for i := range len(digits) {
		if digits[i] < '0' || digits[i] > '9' {
			return false
		}
	}
	_, err := strconv.ParseInt(key, 10, 64)
	return err == nil
}
