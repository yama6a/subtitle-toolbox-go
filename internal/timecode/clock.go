package timecode

import (
	"strings"
	"time"
)

// maxClockDigits is the most digits that the hours or the minutes of Clock can have below MaxTime: 6000000 minutes
// reach it. Fewer digits keep the sums below an int64 overflow.
const maxClockDigits = 7

// Clock reads the time codes h:mm:ss and m:ss, each with an optional fraction after "." or ",", such as
// "01:02:03.456", "00:00:02,500" or "01:02.5". Minutes and seconds after a colon have 2 digits from 00 to 59.
// The minutes of m:ss can be 60 or more. The fraction rounds to whole milliseconds, half away from zero.
// ok is false for another shape, such as a sign or a frame count, and for a time of MaxTime or more.
func Clock(s string) (d time.Duration, ok bool) {
	d, ok = ClockOrLimit(s)
	if !ok || d >= MaxTime {
		return 0, false
	}
	return d, true
}

// ClockOrLimit reads the time codes of Clock. A time of MaxTime or more gives MaxTime, so the caller can name the
// limit in its message. ok is false only for another shape.
func ClockOrLimit(s string) (d time.Duration, ok bool) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	seconds, millis, ok := clockSeconds(parts[len(parts)-1])
	if !ok || !IsDigits(parts[0]) || len(parts) == 3 && !IsSexagesimal(parts[1]) {
		return 0, false
	}
	first := strings.TrimLeft(parts[0], "0")
	if len(first) > maxClockDigits {
		return MaxTime, true
	}
	minutes := int64(atoi(first))
	if len(parts) == 3 {
		minutes = minutes*60 + int64(atoi(parts[1]))
	}
	total := minutes*60 + seconds
	if total >= int64(MaxTime/time.Second) {
		return MaxTime, true
	}
	return min(time.Duration(total)*time.Second+time.Duration(millis)*time.Millisecond, MaxTime), true
}

// clockSeconds reads the 2-digit seconds and the optional fraction of the last part of a time code.
func clockSeconds(s string) (seconds, millis int64, ok bool) {
	whole, fraction, hasFraction := strings.Cut(s, ".")
	if !hasFraction {
		whole, fraction, hasFraction = strings.Cut(s, ",")
	}
	if hasFraction && !IsDigits(fraction) || !IsSexagesimal(whole) {
		return 0, 0, false
	}
	return int64(atoi(whole)), fractionMillis(fraction), true
}

// IsSexagesimal reports whether s is 2 digits from 00 to 59.
func IsSexagesimal(s string) bool {
	return len(s) == 2 && s[0] >= '0' && s[0] <= '5' && IsDigit(s[1])
}

func fractionMillis(fraction string) int64 {
	if len(fraction) <= 3 {
		return int64(atoi(fraction + strings.Repeat("0", 3-len(fraction))))
	}
	millis := int64(atoi(fraction[:3]))
	if fraction[3] >= '5' {
		millis++
	}
	return millis
}
