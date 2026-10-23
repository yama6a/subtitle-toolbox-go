package cli

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
)

// phpSpace holds the white space that a PHP numeric string may have at both ends.
const phpSpace = " \t\n\r\v\f"

// maxIntDigits keeps a whole number below the int64 limit, as PHP keeps it below PHP_INT_MAX.
const maxIntDigits = 18

var (
	// numberPattern is the PHP numeric string. strconv.ParseFloat alone also takes "inf", "nan" and hexadecimal.
	numberPattern = regexp.MustCompile(`^[ \t\n\r\v\f]*[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?[ \t\n\r\v\f]*$`)
	intPattern    = regexp.MustCompile(`^-?[0-9]+$`)
)

// parseNumber returns the value of a PHP numeric string, which may be infinite, such as "1e999".
func parseNumber(s string) (float64, bool) {
	if !numberPattern.MatchString(s) {
		return 0, false
	}
	f, _ := strconv.ParseFloat(strings.Trim(s, phpSpace), 64)
	return f, true
}

// Float returns the number of the option name. ok is false when the call does not give the option.
func (a *args) Float(name string) (f float64, ok bool, err error) {
	v, ok := a.Value(name)
	if !ok {
		return 0, false, nil
	}
	f, isNumber := parseNumber(v)
	if !isNumber {
		return 0, false, usage("The option --" + name + " needs a number, got " + quote(v) + ".")
	}
	if math.IsInf(f, 0) {
		return 0, false, notFinite(name, v)
	}
	return f, true, nil
}

// PositiveFloat returns the number of the option name and fails for 0 and below.
func (a *args) PositiveFloat(name string) (float64, bool, error) {
	f, ok, err := a.Float(name)
	if ok && f <= 0 {
		return 0, false, notPositive(name)
	}
	return f, ok, err
}

// Rate returns the frame rate of the option name, else of --fps, which sets every frame rate. It checks --fps
// first, also when the call gives name, so a bad --fps always fails.
func (a *args) Rate(name string) (float64, bool, error) {
	fps, fpsGiven, err := a.PositiveFloat("fps")
	if err != nil {
		return 0, false, err
	}
	if f, ok, err := a.PositiveFloat(name); err != nil || ok {
		return f, ok, err
	}
	return fps, fpsGiven, nil
}

// NonNegativeFloat returns the number of the option name and fails below 0.
func (a *args) NonNegativeFloat(name string) (float64, bool, error) {
	f, ok, err := a.Float(name)
	if ok && f < 0 {
		return 0, false, negative(name)
	}
	return f, ok, err
}

// Time returns the time of the option name: a number of seconds, or a time code of timecode.Clock with an
// optional leading "-". It rounds to whole milliseconds.
func (a *args) Time(name string) (time.Duration, bool, error) {
	v, ok := a.Value(name)
	if !ok {
		return 0, false, nil
	}
	d, err := parseTime(name, v)
	return d, err == nil, err
}

// parseTime reads v as the time of Time. Messages name the option name.
func parseTime(name, v string) (time.Duration, error) {
	if f, isNumber := parseNumber(v); isNumber {
		if math.IsInf(f, 0) {
			return 0, notFinite(name, v)
		}
		// The limit applies after the rounding, so 359999999.9996 s fails as 100000 h.
		if math.Abs(f) >= timecode.MaxTime.Seconds() || millis(f).Abs() >= timecode.MaxTime {
			return 0, usage("The option --" + name + " needs a time below " + strconv.Itoa(timecode.MaxHours) +
				" hours, got " + quote(v) + ".")
		}
		return millis(f), nil
	}
	clock, negate := strings.CutPrefix(v, "-")
	d, isClock := timecode.Clock(clock)
	if !isClock {
		return 0, usage("The option --" + name + " needs seconds or a timecode such as 00:01:02.500, got " +
			quote(v) + ".")
	}
	if negate {
		d = -d
	}
	return d, nil
}

// PositiveTime returns the time of the option name and fails for 0 and below.
func (a *args) PositiveTime(name string) (time.Duration, bool, error) {
	d, ok, err := a.Time(name)
	if ok && d <= 0 {
		return 0, false, notPositive(name)
	}
	return d, ok, err
}

// NonNegativeTime returns the time of the option name and fails below 0.
func (a *args) NonNegativeTime(name string) (time.Duration, bool, error) {
	d, ok, err := a.Time(name)
	if ok && d < 0 {
		return 0, false, negative(name)
	}
	return d, ok, err
}

// Int returns the whole number of the option name and fails below lo.
func (a *args) Int(name string, lo int64) (int64, bool, error) {
	return a.wholeNumber(name, lo, math.MaxInt64, "of "+strconv.FormatInt(lo, 10)+" or more")
}

// IntRange returns the whole number of the option name and fails outside lo to hi.
func (a *args) IntRange(name string, lo, hi int64) (int64, bool, error) {
	return a.wholeNumber(name, lo, hi, "from "+strconv.FormatInt(lo, 10)+" to "+strconv.FormatInt(hi, 10))
}

// PositiveInt returns the whole number of the option name and fails below 1.
func (a *args) PositiveInt(name string) (int64, bool, error) {
	return a.Int(name, 1)
}

func (a *args) wholeNumber(name string, lo, hi int64, bounds string) (int64, bool, error) {
	v, ok := a.Value(name)
	if !ok {
		return 0, false, nil
	}
	digits := strings.TrimLeft(strings.TrimPrefix(v, "-"), "0")
	if intPattern.MatchString(v) && len(digits) <= maxIntDigits {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= lo && n <= hi {
			return n, true, nil
		}
	}
	return 0, false, usage("The option --" + name + " needs a whole number " + bounds + ", got " + quote(v) + ".")
}

// Choice returns the entry of allowed that the value of the option name matches in any ASCII case.
// strings.EqualFold would also fold U+017F and U+212A, which PHP strcasecmp does not.
func (a *args) Choice(name string, allowed ...string) (string, bool, error) {
	v, ok := a.Value(name)
	if !ok {
		return "", false, nil
	}
	for _, choice := range allowed {
		if asciiLower(choice) == asciiLower(v) {
			return choice, true, nil
		}
	}
	list := allowed[len(allowed)-1]
	if len(allowed) > 1 {
		list = strings.Join(allowed[:len(allowed)-1], ", ") + " or " + list
	}
	return "", false, usage("The option --" + name + " must be " + list + ", got " + quote(v) + ".")
}

func notFinite(name, v string) error {
	return usage("The option --" + name + " needs a finite number, got " + quote(v) + ".")
}

func notPositive(name string) error {
	return usage("The option --" + name + " must be greater than 0.")
}

func negative(name string) error {
	return usage("The option --" + name + " must not be negative.")
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// quote puts v in double quotes as written, as the PHP messages do.
func quote(v string) string {
	return `"` + v + `"`
}
