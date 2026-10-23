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

// MaxValidTime is 99999:59:59.999, the largest cue time below MaxTime.
const MaxValidTime = MaxTime - time.Millisecond

// MaxRetimed is 2562046:59:59.999, the largest time that a word timestamp holds. A retimed time stops there, so it
// stays a time for the next retiming.
const MaxRetimed = 2562047*time.Hour - time.Millisecond

// FromSeconds rounds seconds to whole nanoseconds, then to whole milliseconds, half away from zero. This gives the
// value of PHP round($seconds, 3) for the times of the read goldens. secs must be below MaxTime in size.
func FromSeconds(secs float64) time.Duration {
	return time.Duration(math.Round(secs * 1e9)).Round(time.Millisecond)
}

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

// Units returns d in whole units, rounded half away from zero on whole milliseconds, from 0 up to the largest count
// below MaxTime.
func Units(d, unit time.Duration) int64 {
	return min(int64(max(d, 0).Round(time.Millisecond).Round(unit)/unit), int64((MaxTime-1)/unit))
}

// FromFrames returns the time of frame n at fps frames per second, rounded to whole milliseconds, or MaxTime for a
// time at or beyond the limit.
func FromFrames(n, fps float64) time.Duration {
	secs := n / fps
	// The check comes before the conversion, because a larger value would overflow a time.Duration.
	if secs < MaxTime.Seconds() {
		return FromSeconds(secs)
	}
	return MaxTime
}

// Frames returns the frame of d at fps frames per second, rounded half away from zero on whole milliseconds, from 0
// up to the largest frame whose FromFrames time is below MaxTime.
func Frames(d time.Duration, fps float64) float64 {
	secs := float64(max(min(d, MaxValidTime), 0).Milliseconds()) / 1000
	n := math.Round(secs * fps)
	// Up to 1000 fps a frame is at least 1 ms long, so one frame back is below the limit. Above 1000 fps the
	// rounding to milliseconds never reaches the limit.
	if FromFrames(n, fps) >= MaxTime {
		n--
	}
	return n
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
