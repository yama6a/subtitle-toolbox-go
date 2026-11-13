package model

import (
	"math"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
)

// maxDecimalDigits is the most digits that FrameRateFromFloat accepts, so that the numerator and the denominator fit in an int64.
const maxDecimalDigits = 18

// FrameRate is a frame rate as an exact fraction: Num frames in Den seconds. The zero value means "not set".
type FrameRate struct {
	// Num is the number of frames.
	Num int64
	// Den is the number of seconds that Num frames take.
	Den int64
	// DropFrame selects drop-frame time codes. Validate allows it for 30000/1001, 2997/100, 60000/1001 and 2997/50 only.
	DropFrame bool
}

// FrameRateFromFloat returns the shortest decimal of fps as a reduced fraction, so 23.976 gives 2997/125 and 25 gives 25/1.
// It returns an error that matches ErrInvalidArgument when fps is NaN, infinite, 0 or negative, or has more than 18 digits.
func FrameRateFromFloat(fps float64) (FrameRate, error) {
	if !(fps > 0) || math.IsInf(fps, 1) {
		return FrameRate{}, badFrameRate(fps)
	}
	whole, frac, _ := strings.Cut(strconv.FormatFloat(fps, 'f', -1, 64), ".")
	if len(whole)+len(frac) > maxDecimalDigits {
		return FrameRate{}, invalidArgument("The frame rate must have at most " + strconv.Itoa(maxDecimalDigits) +
			" digits, got " + timecode.Number(fps) + ".")
	}
	num, _ := strconv.ParseInt(strings.TrimLeft(whole+frac, "0"), 10, 64)
	den := int64(1)
	for range frac {
		den *= 10
	}
	g := gcd(num, den)
	return FrameRate{Num: num / g, Den: den / g}, nil
}

// IsZero reports whether r is the zero value, which means "not set".
func (r FrameRate) IsZero() bool {
	return r == FrameRate{}
}

// Float64 returns the frames per second as a float64, and 0 for a zero Den.
func (r FrameRate) Float64() float64 {
	if r.Den == 0 {
		return 0
	}
	return float64(r.Num) / float64(r.Den)
}

// Equal reports whether r and o have the same rate and the same DropFrame, so 25/1 equals 50/2.
func (r FrameRate) Equal(o FrameRate) bool {
	return r.reduced() == o.reduced()
}

// String returns the frames per second with up to 3 decimals and no trailing zeros, such as "23.976" or "25".
func (r FrameRate) String() string {
	s := strconv.FormatFloat(r.Float64(), 'f', 3, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

// Validate returns an error that matches ErrInvalidArgument unless Num and Den are above 0 and DropFrame is allowed.
// DropFrame is allowed for 30000/1001, 2997/100, 60000/1001 and 2997/50 only.
func (r FrameRate) Validate() error {
	if r.Num <= 0 || r.Den <= 0 {
		return badFrameRate(r.number())
	}
	if r.DropFrame && !r.dropFrameAllowed() {
		return invalidArgument("The frame rate must be 30000/1001, 2997/100, 60000/1001 or 2997/50 for drop-frame time codes, got " +
			timecode.Number(r.Float64()) + ".")
	}
	return nil
}

func (r FrameRate) dropFrameAllowed() bool {
	switch r.reduced() {
	case FrameRate{Num: 30000, Den: 1001, DropFrame: true}, FrameRate{Num: 2997, Den: 100, DropFrame: true},
		FrameRate{Num: 60000, Den: 1001, DropFrame: true}, FrameRate{Num: 2997, Den: 50, DropFrame: true}:
		return true
	}
	return false
}

func (r FrameRate) reduced() FrameRate {
	if g := gcd(r.Num, r.Den); g > 1 {
		r.Num /= g
		r.Den /= g
	}
	return r
}

// number returns the rate as a float for a message, without a division by 0.
func (r FrameRate) number() float64 {
	switch {
	case r.Den != 0:
		return r.Float64()
	case r.Num == 0:
		return 0
	}
	return math.Inf(int(r.Num))
}

func badFrameRate(fps float64) error {
	return invalidArgument("The frame rate must be greater than 0, got " + timecode.Number(fps) + ".")
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return max(a, 1)
}
