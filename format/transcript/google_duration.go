package transcript

import (
	"fmt"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
)

// googleOverflow returns a time that wordgroup.Seconds reports as not below 100000 hours.
func googleOverflow() orderedjson.Value { return orderedjson.StringValue("1e30") }

// googleDuration reads a protobuf Duration in JSON as a number of seconds that wordgroup.Seconds and
// wordgroup.SameTime accept. A string such as "1.300s" gives its decimal digits, so the exact value reaches the bounded
// Duration of orderedjson. An object with seconds and nanos gives their sum in whole milliseconds. ok is false for any
// other value. A minus sign is part of a duration only when signed is true.
func googleDuration(v orderedjson.Value, signed bool) (orderedjson.Value, bool) {
	switch v.Kind() {
	case orderedjson.String:
		s, _ := v.AsString()
		digits, found := strings.CutSuffix(s, "s")
		if !found || !googleDecimal(digits, signed) {
			return v, false
		}
		return orderedjson.StringValue(digits), true
	case orderedjson.Object:
		return googleObjectDuration(v)
	case orderedjson.Null, orderedjson.Bool, orderedjson.Number, orderedjson.Array:
	}
	return v, false
}

// googleDecimal reports whether s is digits with an optional fraction, and with an optional minus sign when signed.
func googleDecimal(s string, signed bool) bool {
	if signed {
		s = strings.TrimPrefix(s, "-")
	}
	whole, fraction, hasFraction := strings.Cut(s, ".")
	return googleDigits(whole) && (!hasFraction || googleDigits(fraction))
}

func googleDigits(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}

// googleObjectDuration returns seconds plus nanos divided by 1e9. A missing member counts 0. At least one must be set.
func googleObjectDuration(v orderedjson.Value) (orderedjson.Value, bool) {
	seconds, nanos := cloudGet(v, "seconds"), cloudGet(v, "nanos")
	if seconds.Kind() == orderedjson.Null && nanos.Kind() == orderedjson.Null {
		return v, false
	}
	var total time.Duration
	for _, part := range []struct {
		v    orderedjson.Value
		unit time.Duration
	}{{seconds, time.Second}, {nanos, time.Nanosecond}} {
		switch k := part.v.Kind(); {
		case k == orderedjson.Null:
			continue
		case k != orderedjson.Number && k != orderedjson.String:
			return v, false
		}
		d, ok := part.v.Duration(part.unit)
		if !ok {
			if text, isString := part.v.AsString(); isString {
				if _, numeric := orderedjson.NumericString(text); !numeric {
					return v, false
				}
			}
			return googleOverflow(), true
		}
		total += d
	}
	return orderedjson.Seconds(total), true
}

// googleDurationText writes d as a protobuf Duration with 0 or 3 fraction digits, such as "6s" and "1.500s".
func googleDurationText(d time.Duration) string {
	ms := max(0, d).Round(time.Millisecond).Milliseconds()
	if ms%1000 == 0 {
		return fmt.Sprintf("%ds", ms/1000)
	}
	return fmt.Sprintf("%d.%03ds", ms/1000, ms%1000)
}
