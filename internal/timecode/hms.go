package timecode

import (
	"strconv"
	"strings"
	"time"
)

// maxHourDigits is the digit count above which a time is 100000 hours or more, whatever its other fields.
const maxHourDigits = 6

// HMS returns the time of the digit strings hours, minutes and seconds. Hours of more than 6 digits cannot be below
// 100000, so they give MaxTime and the multiplication cannot wrap.
func HMS(hours, minutes, seconds string) time.Duration {
	hours = strings.TrimLeft(hours, "0")
	if len(hours) > maxHourDigits {
		return MaxTime
	}
	h, _ := strconv.Atoi("0" + hours)
	m, _ := strconv.Atoi(minutes)
	s, _ := strconv.Atoi(seconds)
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second
}
