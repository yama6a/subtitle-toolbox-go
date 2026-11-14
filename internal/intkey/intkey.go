// Package intkey tells which object keys of format data read as list indexes.
package intkey

import (
	"strconv"
	"strings"
)

// IsIntKey reports whether the key is a canonical decimal int64, so it reads as a list index.
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
