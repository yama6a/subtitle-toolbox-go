package orderedjson

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// maxDuration is the limit of a time: 100,000 hours. A value at or beyond it, in either sign, has no Duration.
const maxDuration = 100000 * time.Hour

func validNumber(s string) bool {
	d := &decoder{data: []byte(s)}
	if s == "" || (s[0] != '-' && !isDigit(s[0])) {
		return false
	}
	_, err := d.number()
	return err == nil && d.pos == len(s)
}

// NumericString reports whether s is numeric as PHP is_numeric says, and returns its text without the
// surrounding white space.
func NumericString(s string) (string, bool) {
	s = strings.Trim(s, " \t\n\r\v\f")
	_, ok := parseDecimal(s)
	return s, ok
}

// decimal is a number as a sign, digits without leading zeros and a power of ten: value = digits * 10^exp.
type decimal struct {
	neg    bool
	digits string
	exp    int
}

// maxExp clamps the exponent text so that it cannot overflow int.
const maxExp = 1_000_000_000

// parseDecimal reads a PHP numeric text: an optional sign, digits with an optional fraction or a fraction alone,
// and an optional exponent.
func parseDecimal(s string) (decimal, bool) {
	var d decimal
	if s != "" && (s[0] == '+' || s[0] == '-') {
		d.neg = s[0] == '-'
		s = s[1:]
	}
	intPart, s := leadingDigits(s)
	var frac string
	if strings.HasPrefix(s, ".") {
		frac, s = leadingDigits(s[1:])
	}
	if intPart == "" && frac == "" {
		return decimal{}, false
	}
	exp, s := parseExponent(s)
	if s != "" {
		return decimal{}, false
	}
	d.digits = strings.TrimLeft(intPart+frac, "0")
	d.exp = exp - len(frac)
	return d, true
}

func leadingDigits(s string) (string, string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

// parseExponent reads an optional exponent from the start of s, clamped to plus or minus maxExp.
// Without digits after the e it reads nothing.
func parseExponent(s string) (int, string) {
	if s == "" || (s[0] != 'e' && s[0] != 'E') {
		return 0, s
	}
	rest := s[1:]
	neg := strings.HasPrefix(rest, "-")
	if neg || strings.HasPrefix(rest, "+") {
		rest = rest[1:]
	}
	digits, rest := leadingDigits(rest)
	if digits == "" {
		return 0, s
	}
	digits = strings.TrimLeft(digits, "0")
	exp := 0
	switch {
	case len(digits) > 9:
		exp = maxExp
	case digits != "":
		exp = atoi(digits)
	}
	if neg {
		exp = -exp
	}
	return exp, rest
}

func atoi(digits string) int {
	n := 0
	for _, c := range digits {
		n = n*10 + int(c-'0')
	}
	return n
}

// Duration returns the number times unit, rounded half away from zero to whole milliseconds.
// v is a number, or a string that NumericString accepts. It fails for a value that is not finite
// and for a result of 100,000 hours or more. A value below half a millisecond is 0.
func (v Value) Duration(unit time.Duration) (time.Duration, bool) {
	text := v.text
	switch {
	case v.kind == Number && !v.flag:
	case v.kind == String:
		text, _ = NumericString(v.text)
	default:
		return 0, false
	}
	d, ok := parseDecimal(text)
	if !ok || unit <= 0 {
		return 0, false
	}
	return d.milliseconds(unit)
}

func (d decimal) milliseconds(unit time.Duration) (time.Duration, bool) {
	if d.digits == "" {
		return 0, true
	}
	// value in milliseconds = digits * mult * 10^shift, where unit = mult * 10^zeros nanoseconds.
	mult, zeros := int64(unit), 0
	for mult%10 == 0 {
		mult /= 10
		zeros++
	}
	if mult > maxMultiplier {
		return 0, false
	}
	shift := d.exp + zeros - 6
	switch {
	case shift >= 0 && len(d.digits)+shift > maxDigits:
		return 0, false
	case -shift > len(d.digits)+len(strconv.FormatInt(mult, 10)):
		return 0, true
	}
	ms, ok := roundedMillis(mulDigits(d.digits, mult), shift)
	if !ok {
		return 0, false
	}
	if ms >= int64(maxDuration/time.Millisecond) {
		return 0, false
	}
	if d.neg {
		ms = -ms
	}
	return time.Duration(ms) * time.Millisecond, true
}

// roundedMillis returns digits * 10^shift rounded half up, or false when it has more than maxDigits digits.
func roundedMillis(digits string, shift int) (int64, bool) {
	if shift >= 0 {
		if len(digits)+shift > maxDigits {
			return 0, false
		}
		return atoi64(digits + strings.Repeat("0", shift)), true
	}
	cut := len(digits) + shift
	switch {
	case cut < 0:
		return 0, true
	case cut > maxDigits:
		return 0, false
	}
	ms := atoi64("0" + digits[:cut])
	if digits[cut] >= '5' {
		ms++
	}
	return ms, true
}

// maxDigits is the most digits of a result in milliseconds that can stay below maxDuration.
const maxDigits = 12

// maxMultiplier is the largest unit, after its trailing zeros are cut, that mulDigits multiplies without overflow.
const maxMultiplier = 1_000_000_000_000_000

// mulDigits returns the decimal digits times m, as digits without leading zeros. m must be positive.
func mulDigits(digits string, m int64) string {
	out := make([]byte, len(digits)+len(strconv.FormatInt(m, 10)))
	var carry uint64
	for i, j := len(digits)-1, len(out)-1; j >= 0; i, j = i-1, j-1 {
		if i >= 0 {
			carry += uint64(digits[i]-'0') * uint64(m)
		}
		out[j] = byte('0' + carry%10)
		carry /= 10
	}
	return strings.TrimLeft(string(out), "0")
}

func atoi64(digits string) int64 {
	var n int64
	for _, c := range digits {
		n = n*10 + int64(c-'0')
	}
	return n
}

// formatFloat prints f in the layout of Python's json module.
func formatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'e', -1, 64)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	mant, expText, _ := strings.Cut(s, "e")
	exp, _ := strconv.Atoi(expText)
	digits := strings.Replace(mant, ".", "", 1)
	var out string
	switch {
	case exp < -4 || exp >= 16:
		out = digits[:1]
		if len(digits) > 1 {
			out += "." + digits[1:]
		}
		sign := '+'
		if exp < 0 {
			sign = '-'
			exp = -exp
		}
		out += fmt.Sprintf("e%c%02d", sign, exp)
	case exp < 0:
		out = "0." + strings.Repeat("0", -exp-1) + digits
	case len(digits) <= exp+1:
		out = digits + strings.Repeat("0", exp+1-len(digits)) + ".0"
	default:
		out = digits[:exp+1] + "." + digits[exp+1:]
	}
	if neg {
		return "-" + out
	}
	return out
}
