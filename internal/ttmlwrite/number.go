package ttmlwrite

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

var (
	phpFloatRegexp   = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?`)
	multiplierRegexp = regexp.MustCompile(`[\t\n\v\f\r :]+`)
)

// PHPFloat reads s as the PHP cast (float) does: leading white space, then the longest decimal number, else 0.
func PHPFloat(s string) float64 {
	m := phpFloatRegexp.FindString(strings.TrimLeft(s, " \t\n\r\v\f"))
	v, _ := strconv.ParseFloat(m, 64)
	return v
}

// Multiplier reads the value of ttp:frameRateMultiplier as the 2 numbers a and b. A value that is not 2 numbers above 0,
// split at white space or colons, gives 1 and 1.
func Multiplier(v string) (a, b float64) {
	parts := multiplierRegexp.Split(strings.Trim(v, markupcore.PHPTrimSet), -1)
	if len(parts) != 2 || PHPFloat(parts[0]) <= 0 || PHPFloat(parts[1]) <= 0 {
		return 1, 1
	}
	return PHPFloat(parts[0]), PHPFloat(parts[1])
}
