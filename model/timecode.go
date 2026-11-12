package model

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
)

// labelsPerDropped is the labels per second for each label that drop-frame skips at a minute: 2 at 30, 4 at 60.
const labelsPerDropped = 15

var (
	timecodeLabel = regexp.MustCompile(`^(\d{2,}):(\d{2}):(\d{2})([:;])(\d{2})$`)
	timeFrames    = regexp.MustCompile(`^(\d+):([0-5]\d):([0-5]\d):(\d{2,})$`)
)

// Timecode is a SMPTE time code label, such as "01:00:00;00".
type Timecode struct {
	Hours   int
	Minutes int
	Seconds int
	Frames  int
	// DropFrame is true when the label has ";" before the frames.
	DropFrame bool
}

// ParseTimecode reads the label HH:MM:SS:FF or HH:MM:SS;FF. It checks no range of the fields.
// It returns an error that matches ErrInvalidArgument for another shape.
func ParseTimecode(s string) (Timecode, error) {
	m := timecodeLabel.FindStringSubmatch(s)
	if m == nil {
		return Timecode{}, badTimecodeLabel(s)
	}
	hours, err := strconv.Atoi(m[1])
	if err != nil {
		return Timecode{}, badTimecodeLabel(s)
	}
	minutes, _ := strconv.Atoi(m[2])
	seconds, _ := strconv.Atoi(m[3])
	frames, _ := strconv.Atoi(m[5])
	return Timecode{Hours: hours, Minutes: minutes, Seconds: seconds, Frames: frames, DropFrame: m[4] == ";"}, nil
}

func badTimecodeLabel(s string) error {
	return invalidArgument(`The time code "` + s + `" is not HH:MM:SS:FF or HH:MM:SS;FF.`)
}

// String returns the label, with ";" before the frames for drop-frame and ":" otherwise.
func (t Timecode) String() string {
	sep := byte(':')
	if t.DropFrame {
		sep = ';'
	}
	b := appendPadded(nil, t.Hours)
	b = append(b, ':')
	b = appendPadded(b, t.Minutes)
	b = append(b, ':')
	b = appendPadded(b, t.Seconds)
	b = append(b, sep)
	return string(appendPadded(b, t.Frames))
}

func appendPadded(b []byte, n int) []byte {
	if n >= 0 && n < 10 {
		b = append(b, '0')
	}
	return strconv.AppendInt(b, int64(n), 10)
}

// Labels returns Num/Den rounded half away from zero, so 30000/1001 gives 30 and 25/2 gives 13.
// It returns 0 when Num or Den is 0 or below.
func (r FrameRate) Labels() int {
	if r.Num <= 0 || r.Den <= 0 {
		return 0
	}
	n := r.Num / r.Den
	if rem := r.Num % r.Den; rem >= r.Den-rem {
		n++
	}
	return int(n)
}

// Frames returns the frame nearest to d, which is d * Num / Den rounded half away from zero.
// It returns 0 for a rate that fails Validate.
func (r FrameRate) Frames(d time.Duration) int64 {
	if r.Num <= 0 || r.Den <= 0 {
		return 0
	}
	if r.Den <= math.MaxInt64/int64(time.Second) {
		if n, ok := smallMulDiv(int64(d), r.Num, r.Den*int64(time.Second)); ok {
			return n
		}
	}
	return roundDiv(product(int64(d), r.Num), product(r.Den, int64(time.Second)))
}

// Duration returns the exact time of frame n, which is n * Den / Num seconds rounded to the nearest nanosecond.
// It returns 0 for a rate that fails Validate.
func (r FrameRate) Duration(n int64) time.Duration {
	if r.Num <= 0 || r.Den <= 0 {
		return 0
	}
	if p, ok := smallProduct(n, r.Den); ok {
		if q, ok := smallMulDiv(p, int64(time.Second), r.Num); ok {
			return time.Duration(q)
		}
	}
	return time.Duration(roundDiv(product(n, r.Den, int64(time.Second)), product(r.Num)))
}

// smallProduct returns a * b when both are 0 or more and the product fits in an int64.
func smallProduct(a, b int64) (int64, bool) {
	if a < 0 || b < 0 || (a != 0 && b > math.MaxInt64/a) {
		return 0, false
	}
	return a * b, true
}

// smallMulDiv returns a * b / den rounded half away from zero, as roundDiv does, without big integers. It reports
// false when an operand is negative, den is 0 or below, or a value does not fit in an int64.
func smallMulDiv(a, b, den int64) (int64, bool) {
	num, ok := smallProduct(a, b)
	if !ok || den <= 0 || num > math.MaxInt64-den>>1 {
		return 0, false
	}
	return (num + den>>1) / den, true
}

// Near reports whether r and o differ by 0.01 frames per second or less. It ignores DropFrame.
func (r FrameRate) Near(o FrameRate) bool {
	d := r.Float64() - o.Float64()
	return d <= 0.01 && d >= -0.01
}

// TimecodeOf returns the label of frame n in the frame-count model, with drop-frame labels when r.DropFrame is set.
// A negative n gives 00:00:00:00. A zero rate gives the zero Timecode.
func (r FrameRate) TimecodeOf(n int64) Timecode {
	labels := int64(r.Labels())
	if labels == 0 {
		return Timecode{}
	}
	n = max(n, 0)
	if r.DropFrame {
		dropped := labels / labelsPerDropped
		perCycle := 600*labels - 9*dropped
		perMinute := 60*labels - dropped
		rest := n % perCycle
		n += 9 * dropped * (n / perCycle)
		if rest >= dropped {
			n += dropped * ((rest - dropped) / perMinute)
		}
	}
	t := splitSeconds(n/labels, int(n%labels))
	t.DropFrame = r.DropFrame
	return t
}

// FrameOf returns the frame of label t in the frame-count model, the inverse of TimecodeOf.
// It reads t.DropFrame never: r.DropFrame decides. It checks no range of the fields.
// A label that drop-frame skips gives the frame of the label before it.
func (r FrameRate) FrameOf(t Timecode) int64 {
	labels := int64(r.Labels())
	minutes := int64(t.Hours)*60 + int64(t.Minutes)
	n := (minutes*60+int64(t.Seconds))*labels + int64(t.Frames)
	if r.DropFrame {
		n -= labels / labelsPerDropped * (minutes - minutes/10)
	}
	return n
}

// ClockTimecode returns the label of d in the clock model: the whole clock seconds of d, then the frame inside the
// last second. It rounds d to whole milliseconds first and clamps it at 0. A frame at or above Labels rolls into the
// next second as frame 0. The label is never drop-frame. A zero rate gives the zero Timecode.
func (r FrameRate) ClockTimecode(d time.Duration) Timecode {
	labels := r.Labels()
	if labels == 0 {
		return Timecode{}
	}
	d = max(d.Round(time.Millisecond), 0)
	whole := int64(d / time.Second)
	frame := int(r.Frames(d % time.Second))
	if frame >= labels {
		whole++
		frame = 0
	}
	return splitSeconds(whole, frame)
}

// ClockDuration returns the time of t in the clock model: hours, minutes and seconds plus Frames * Den / Num seconds.
// It checks no range of the fields. A zero rate gives the time without the frames.
func (r FrameRate) ClockDuration(t Timecode) time.Duration {
	d := (time.Duration(t.Hours)*60+time.Duration(t.Minutes))*time.Minute + time.Duration(t.Seconds)*time.Second
	return d + r.Duration(int64(t.Frames))
}

func splitSeconds(seconds int64, frames int) Timecode {
	return Timecode{
		Hours:   int(seconds / 3600),
		Minutes: int(seconds % 3600 / 60),
		Seconds: int(seconds % 60),
		Frames:  frames,
	}
}

// product multiplies without overflow, so a large Den cannot wrap around.
func product(xs ...int64) *big.Int {
	p := big.NewInt(1)
	for _, x := range xs {
		p.Mul(p, big.NewInt(x))
	}
	return p
}

// roundDiv returns num / den rounded half away from zero. den must be above 0. A result beyond the int64 range
// stops at the limit.
func roundDiv(num, den *big.Int) int64 {
	neg := num.Sign() < 0
	x := new(big.Int).Abs(num)
	x.Add(x, new(big.Int).Rsh(den, 1))
	x.Quo(x, den)
	if neg {
		x.Neg(x)
	}
	if !x.IsInt64() {
		if neg {
			return -1 << 63
		}
		return 1<<63 - 1
	}
	return x.Int64()
}

// ParseTime returns the time of a time string, rounded to whole milliseconds. It reads 3 shapes:
// h:mm:ss with an optional fraction after "." or ",", such as "00:01:02,500", m:ss with an optional fraction, such as
// "01:02.5", and h:mm:ss:ff in the clock model, such as "00:01:02:12" at 25 fps. Only the last shape needs r.
// It returns an *InvalidArgumentError for another shape, for a time of 100000 hours or more, for frames without a
// valid r, and for a frame number at or above r.Labels.
func ParseTime(s string, r FrameRate) (time.Duration, error) {
	if d, ok := timecode.ClockOrLimit(s); ok {
		if d >= timecode.MaxTime {
			return 0, timeBelowLimit(s)
		}
		return d, nil
	}
	m := timeFrames.FindStringSubmatch(s)
	if m == nil {
		return 0, invalidArgument(`The timecode "` + s + `" is not h:mm:ss.mmm, m:ss.mmm or h:mm:ss:ff.`)
	}
	labels := r.Labels()
	if labels == 0 {
		return 0, invalidArgument(`The timecode "` + s + `" counts frames and needs a frame rate.`)
	}
	hours := digitsValue(m[1])
	if hours >= timecode.MaxHours {
		return 0, timeBelowLimit(s)
	}
	frames := digitsValue(m[4])
	if frames >= int64(labels) {
		return 0, invalidArgument(`The timecode "` + s + `" has a frame number that is not below ` + strconv.Itoa(labels) + ".")
	}
	t := Timecode{Hours: int(hours), Minutes: int(digitsValue(m[2])), Seconds: int(digitsValue(m[3])), Frames: int(frames)}
	d := r.ClockDuration(t).Round(time.Millisecond)
	if d >= timecode.MaxTime {
		return 0, timeBelowLimit(s)
	}
	return d, nil
}

func timeBelowLimit(s string) error {
	return invalidArgument(`The timecode "` + s + `" is not below ` + strconv.Itoa(timecode.MaxHours) + " hours.")
}

// digitsValue returns the value of a string of digits, and a value above any limit for 18 digits or more.
func digitsValue(s string) int64 {
	for len(s) > 1 && s[0] == '0' {
		s = s[1:]
	}
	if len(s) > 17 {
		return 1 << 62
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
