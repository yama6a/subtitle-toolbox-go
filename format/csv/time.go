package csv

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// maxHourDigits is the most hour digits below 100000 hours after leading zeros.
const maxHourDigits = 6

// parseTime reads a trimmed time cell: seconds, h:mm:ss.mmm, m:ss.mmm, hh:mm:ss,mmm or hh:mm:ss:ff. The last
// seconds field may have 1 digit. With millis, a number counts milliseconds. Frames need fps.
func parseTime(t string, fps float64, millis bool, line int) (time.Duration, error) {
	if isDecimal(t) {
		secs, _ := strconv.ParseFloat(t, 64) // a value beyond the float range is +Inf, which the bound rejects
		if millis {
			secs /= 1000
		}
		return parsing.BoundedTime(timecode.FromFrames(secs, 1), t, line)
	}
	if fields, ok := frameFields(t); ok {
		if fps == 0 {
			return 0, &model.ParseError{
				Line: line, Msg: `The time "` + t + `" counts frames and needs a frame rate.`, Option: "ReadOptions.CSV.FrameRate",
			}
		}
		return parsing.BoundedTime(frameTime(fields, fps), t, line)
	}
	if d, ok := timecode.ClockOrLimit(padSeconds(t)); ok {
		return parsing.BoundedTime(d, t, line)
	}
	return 0, &model.ParseError{
		Line: line, Msg: `The time "` + t + `" is not seconds, h:mm:ss.mmm, m:ss.mmm, hh:mm:ss,mmm or hh:mm:ss:ff.`,
	}
}

// isDecimal reports whether t matches ^\d+(?:\.\d+)?$.
func isDecimal(t string) bool {
	whole, fraction, hasFraction := strings.Cut(t, ".")
	return timecode.IsDigits(whole) && (!hasFraction || timecode.IsDigits(fraction))
}

// frameFields splits t of the shape ^(\d+):([0-5]\d):([0-5]\d):(\d+)$.
func frameFields(t string) ([4]string, bool) {
	parts := strings.Split(t, ":")
	if len(parts) != 4 || !timecode.IsDigits(parts[0]) || !timecode.IsSexagesimal(parts[1]) || !timecode.IsSexagesimal(parts[2]) || !timecode.IsDigits(parts[3]) {
		return [4]string{}, false
	}
	return [4]string(parts), true
}

// frameTime returns the whole seconds of the fields plus the frames divided by fps. The frame number is not
// checked against the rate. Hours of more than 6 digits give timecode.MaxTime, so the sum cannot wrap. Frame
// digits beyond the int64 range read as its maximum, as PHP (int) reads them.
func frameTime(fields [4]string, fps float64) time.Duration {
	hours := strings.TrimLeft(fields[0], "0")
	if len(hours) > maxHourDigits {
		return timecode.MaxTime
	}
	h, _ := strconv.ParseInt("0"+hours, 10, 64)
	m, _ := strconv.ParseInt(fields[1], 10, 64)
	s, _ := strconv.ParseInt(fields[2], 10, 64)
	frames, _ := strconv.ParseInt(fields[3], 10, 64)
	return timecode.FromFrames(float64(h*3600+m*60+s)+float64(frames)/fps, 1)
}

// padSeconds puts a 0 before each single digit that follows a colon and ends t or comes before "." or ",", as
// the PHP pattern :(\d)(?=$|[.,]) does. So 00:00:1 becomes 00:00:01.
func padSeconds(t string) string {
	var b strings.Builder
	for i := range len(t) {
		b.WriteByte(t[i])
		if t[i] == ':' && i+1 < len(t) && timecode.IsDigit(t[i+1]) && (i+2 == len(t) || t[i+2] == '.' || t[i+2] == ',') {
			b.WriteByte('0')
		}
	}
	return b.String()
}

// timeFormatOf returns the stored time format of the first parsed start cell, or of a millisecond table.
func timeFormatOf(t string, millis bool) TimeFormat {
	switch {
	case millis:
		return TimeMilliseconds
	case strings.Contains(t, ","):
		return TimeComma
	case strings.Count(t, ":") == 3:
		return TimeFrames
	case strings.Contains(t, ":"):
		return TimeDot
	default:
		return TimeSeconds
	}
}

// timeCell writes d in the time format f. A negative time writes as 0.
func timeCell(d time.Duration, f TimeFormat, fps float64) string {
	d = max(d, 0)
	ms := d.Round(time.Millisecond).Milliseconds()
	switch f {
	case TimeSeconds:
		s := strconv.FormatInt(ms/1000, 10) + "." + pad3(ms%1000)
		return strings.TrimRight(strings.TrimRight(s, "0"), ".")
	case TimeComma:
		return string(timecode.AppendClock(nil, d, 2, ','))
	case TimeFrames:
		return frameCell(ms, fps)
	case TimeMilliseconds:
		return strconv.FormatInt(ms, 10)
	case TimeDot, TimeFormatDefault:
	}
	return markupcore.CoreTimestamp(d)
}

// frameCell writes whole clock seconds and the frames after them. A frame at or above the rounded frame rate
// moves to the next second as frame 0. A time that rounds to the limit writes the last frame below it.
func frameCell(ms int64, fps float64) string {
	ms = min(ms, timecode.MaxValidTime.Milliseconds())
	whole := ms / 1000
	labels := math.Round(fps)
	frame := math.Round(float64(ms%1000) / 1000 * fps)
	if frame >= labels {
		whole++
		frame = 0
	}
	// The last frame of a second is at least 1 ms before its end up to 1000 fps, so it stays below the limit.
	if time.Duration(whole)*time.Second+timecode.FromFrames(frame, fps) >= timecode.MaxTime {
		whole--
		frame = labels - 1
	}
	return pad2(whole/3600) + ":" + pad2(whole/60%60) + ":" + pad2(whole%60) + ":" + pad2(int64(frame))
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

func pad3(n int64) string {
	s := strconv.FormatInt(n, 10)
	return strings.Repeat("0", 3-len(s)) + s
}
