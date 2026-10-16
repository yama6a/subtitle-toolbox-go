// Package timecode holds the limit of cue times and the number format of times in messages.
package timecode

import (
	"math"
	"strconv"
	"time"
)

// MaxHours is the limit of cue times in hours. A cue time must be below it.
const MaxHours = 100_000

// MaxTime is MaxHours as a duration.
const MaxTime = MaxHours * time.Hour

// Seconds writes d in seconds as PHP prints a float, so 3 s gives "3" and 1.5 s gives "1.5".
func Seconds(d time.Duration) string {
	return Number(d.Seconds())
}

// Number writes f as PHP prints a float: 14 significant digits, no trailing zeros, and "NAN", "INF" or "-INF".
// A value from 1e14 on or below 1e-4 in size gets the exponent form of strconv, such as "8e+300" where PHP writes "8.0E+300".
func Number(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NAN"
	case math.IsInf(f, 1):
		return "INF"
	case math.IsInf(f, -1):
		return "-INF"
	}
	return strconv.FormatFloat(f, 'g', 14, 64)
}

// AppendClock appends d as h:mm:ss, the separator sep and mmm, with at least hourWidth hour digits.
// It rounds d to whole milliseconds, half away from zero. A negative time writes as 0.
func AppendClock(dst []byte, d time.Duration, hourWidth int, sep byte) []byte {
	ms := max(d.Round(time.Millisecond).Milliseconds(), 0)
	dst = appendPadded(dst, ms/int64(time.Hour/time.Millisecond), hourWidth)
	dst = append(dst, ':')
	dst = appendPadded(dst, ms/int64(time.Minute/time.Millisecond)%60, 2)
	dst = append(dst, ':')
	dst = appendPadded(dst, ms/int64(time.Second/time.Millisecond)%60, 2)
	dst = append(dst, sep)
	return appendPadded(dst, ms%1000, 3)
}

func appendPadded(dst []byte, n int64, width int) []byte {
	s := strconv.FormatInt(n, 10)
	for range width - len(s) {
		dst = append(dst, '0')
	}
	return append(dst, s...)
}
