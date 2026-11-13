package orderedjson

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// maxDuration is the limit of a time: 100,000 hours. A value at or beyond it, in either sign, has no Duration.
const maxDuration = 100000 * time.Hour

// keptDigits is the number of significant digits of a decimal that Duration reads exactly. It must exceed the
// digits of maxDuration in milliseconds plus the digits of the largest unit in nanoseconds.
const keptDigits = 40

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
	unitDigits := len(strconv.FormatInt(int64(unit), 10))
	// The result lies in [10^(n+unitDigits+exp-8), 10^(n+unitDigits+exp-6)) milliseconds.
	order := len(d.digits) + unitDigits + d.exp
	if order-8 >= 12 {
		return 0, false
	}
	if order-6 < 0 {
		return 0, true
	}
	n := d.truncated().scaled(unit)
	if !n.IsInt64() || n.Int64() >= int64(maxDuration/time.Millisecond) {
		return 0, false
	}
	ms := time.Duration(n.Int64()) * time.Millisecond
	if d.neg {
		ms = -ms
	}
	return ms, true
}

// truncated returns d with at most keptDigits digits. A nonzero tail only matters as "above the kept digits",
// so one digit 1 stands for it.
func (d decimal) truncated() decimal {
	if len(d.digits) <= keptDigits {
		return d
	}
	tail := d.digits[keptDigits:]
	out := decimal{neg: d.neg, digits: d.digits[:keptDigits], exp: d.exp + len(tail)}
	if strings.Trim(tail, "0") != "" {
		out.digits += "1"
		out.exp--
	}
	return out
}

// scaled returns the value in milliseconds as an integer, rounded half up. d must have at most keptDigits+1 digits.
func (d decimal) scaled(unit time.Duration) *big.Int {
	num, _ := new(big.Int).SetString(d.digits, 10)
	num.Mul(num, big.NewInt(int64(unit)))
	shift := d.exp - 6
	if shift >= 0 {
		return num.Mul(num, pow10(shift))
	}
	div := pow10(-shift)
	q, r := new(big.Int).QuoRem(num, div, new(big.Int))
	if r.Lsh(r, 1).Cmp(div) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}

func pow10(n int) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil) }

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
