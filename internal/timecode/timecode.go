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

// MaxRetimed is 2562046:59:59.999, the largest time that a word timestamp holds. A retimed time stops there, so it
// stays a time for the next retiming.
const MaxRetimed = 2562047*time.Hour - time.Millisecond

// TimeLimitMessage returns the message of a retiming that moves the end of cue number n, counted from 1, to end
// seconds, at or beyond MaxTime.
func TimeLimitMessage(n int, end float64) string {
	return "The retimed cue " + strconv.Itoa(n) + " would end at " + Number(end) + " s. Cue times must be below " +
		strconv.Itoa(MaxHours) + " hours."
}

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
