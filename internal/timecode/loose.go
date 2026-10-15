package timecode

import (
	"strings"
	"time"
)

const (
	maxLooseHourDigits     = 5
	maxLooseFractionDigits = 4
)

// Loose reads the time variants that lenient readers repair, such as "0:0:1", "00:00:01,5000" or "00:00:01:50".
// It reads 1 or 2 digit minutes and seconds below 60 and hours of up to 5 digits after leading zeros.
// The fraction has 1 to 4 digits after one of the bytes of separators and rounds to whole milliseconds,
// half away from zero. With hoursOptional, a time without hours such as "07:03,920" reads too.
// When both forms match, the form with hours wins.
func Loose(s, separators string, hoursOptional bool) (time.Duration, bool) {
	if d, ok := loose(s, separators, true); ok || !hoursOptional {
		return d, ok
	}
	return loose(s, separators, false)
}

func loose(s, separators string, withHours bool) (time.Duration, bool) {
	var hours int
	if withHours {
		var ok bool
		if hours, s, ok = looseHours(s); !ok {
			return 0, false
		}
	}
	minutes, rest, ok := sexagesimal(s)
	if !ok || !strings.HasPrefix(rest, ":") {
		return 0, false
	}
	seconds, rest, ok := sexagesimal(rest[1:])
	if !ok {
		return 0, false
	}
	tenths, ok := looseFraction(rest, separators)
	if !ok {
		return 0, false
	}
	millis := (tenths + 5) / 10
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second + time.Duration(millis)*time.Millisecond, true
}

// looseHours reads the hours and their colon at the start of s.
func looseHours(s string) (int, string, bool) {
	h, rest, ok := strings.Cut(s, ":")
	digits := strings.TrimLeft(h, "0")
	if digits == "" && h != "" {
		digits = "0"
	}
	if !ok || !isDigits(h) || len(digits) > maxLooseHourDigits {
		return 0, s, false
	}
	return atoi(digits), rest, true
}

// looseFraction reads an optional separator and fraction, in tenths of a millisecond.
func looseFraction(s, separators string) (int, bool) {
	if s == "" {
		return 0, true
	}
	fraction := s[1:]
	if !strings.Contains(separators, s[:1]) || len(fraction) > maxLooseFractionDigits || !isDigits(fraction) {
		return 0, false
	}
	return atoi(fraction + strings.Repeat("0", maxLooseFractionDigits-len(fraction))), true
}

// sexagesimal reads a field of 1 digit, or of 2 digits below 60, at the start of s.
func sexagesimal(s string) (int, string, bool) {
	n := 0
	for n < len(s) && isDigit(s[n]) {
		n++
	}
	if n == 0 || n > 2 || n == 2 && s[0] > '5' {
		return 0, s, false
	}
	return atoi(s[:n]), s[n:], true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// atoi reads a string of at most 9 digits.
func atoi(s string) int {
	n := 0
	for i := range len(s) {
		n = n*10 + int(s[i]-'0')
	}
	return n
}
