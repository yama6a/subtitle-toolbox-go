package orderedjson_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/orderedjson"
)

func TestNumberAccessors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text    string
		i       int64
		intOK   bool
		f       float64
		floatOK bool
	}{
		{"12", 12, true, 12, true},
		{"-0", 0, true, 0, true},
		{"12.0", 0, false, 12, true},
		{"1e3", 0, false, 1000, true},
		{"9223372036854775808", 0, false, 9.223372036854776e18, true},
		{"1e400", 0, false, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()

			v, err := orderedjson.NumberText(tt.text)
			require.NoError(t, err)
			i, ok := v.AsInt64()
			assert.Equal(t, tt.intOK, ok)
			assert.Equal(t, tt.i, i)
			f, ok := v.AsFloat64()
			assert.Equal(t, tt.floatOK, ok)
			assert.InDelta(t, tt.f, f, 0)
		})
	}

	for _, bad := range []string{"", "01", "1.", ".5", "+1", "1e", "0x1", "NaN", " 1"} {
		_, err := orderedjson.NumberText(bad)
		require.ErrorIs(t, err, orderedjson.ErrInvalidNumber, bad)
	}
}

func TestDuration(t *testing.T) {
	t.Parallel()

	num := func(s string) orderedjson.Value {
		v, err := orderedjson.NumberText(s)
		require.NoError(t, err)
		return v
	}
	tests := []struct {
		name string
		v    orderedjson.Value
		unit time.Duration
		want time.Duration
		ok   bool
	}{
		{"32-bit float time", num("2.8399999141693115"), time.Second, 2840 * time.Millisecond, true},
		{"float multiply would round down", num("1.0005"), time.Second, 1001 * time.Millisecond, true},
		{"milliseconds", num("1234.5"), time.Millisecond, 1235 * time.Millisecond, true},
		{"negative half away from zero", num("-0.0005"), time.Second, -time.Millisecond, true},
		{"numeric string", orderedjson.StringValue("2.0"), time.Second, 2 * time.Second, true},
		{"padded numeric string", orderedjson.StringValue(" .5 "), time.Second, 500 * time.Millisecond, true},
		{"exponent", num("1.5e2"), time.Second, 150 * time.Second, true},
		{"not finite", num("1e400"), time.Second, 0, false},
		{"huge exponent", num("1e1000000000"), time.Second, 0, false},
		{"huge exponent beyond int", num("1e99999999999999999999"), time.Second, 0, false},
		{"tiny exponent", num("1e-1000000000"), time.Second, 0, true},
		{"100000 hours", num("360000000"), time.Second, 0, false},
		{"just below 100000 hours", num("359999999.999"), time.Second, 359999999999 * time.Millisecond, true},
		{"minus 100000 hours", num("-360000000"), time.Second, 0, false},
		{"zero with a long exponent", num("0e999999999"), time.Second, 0, true},
		{"long mantissa", num("0." + strings.Repeat("0", 100) + "5"), time.Second, 0, true},
		{"long mantissa above a tie", num("0.0005" + strings.Repeat("0", 100) + "1"), time.Second, time.Millisecond, true},
		{"long mantissa at a tie", num("0.0005" + strings.Repeat("0", 100)), time.Second, time.Millisecond, true},
		{"string that is not numeric", orderedjson.StringValue("0x1A"), time.Second, 0, false},
		{"not a number", orderedjson.BoolValue(true), time.Second, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := tt.v.Duration(tt.unit)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNumericString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{" 1.5", "1.5", true},
		{"1.5 ", "1.5", true},
		{"\v1\f", "1", true},
		{".5", ".5", true},
		{"5.", "5.", true},
		{"+1", "+1", true},
		{"1e3", "1e3", true},
		{"0x1A", "", false},
		{"inf", "", false},
		{"nan", "", false},
		{"1e", "", false},
		{".", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, ok := orderedjson.NumericString(tt.in)
			assert.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestFloatPrinter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		f    float64
		want string
	}{
		{0, "0.0"},
		{math.Copysign(0, -1), "-0.0"},
		{4, "4.0"},
		{2.34, "2.34"},
		{0.0001, "0.0001"},
		{0.0000123, "1.23e-05"},
		{1e-7, "1e-07"},
		{1e15, "1000000000000000.0"},
		{1e16, "1e+16"},
		{1.5e300, "1.5e+300"},
		{1e-300, "1e-300"},
		{13.120000000000001, "13.120000000000001"},
		{-0.5, "-0.5"},
		{123456.789, "123456.789"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			out, err := orderedjson.Encode(orderedjson.Float(tt.f), false)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(out))
		})
	}

	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := orderedjson.Encode(orderedjson.Float(f), false)
		require.ErrorIs(t, err, orderedjson.ErrNotFinite)
	}
}

func TestSecondsAndInt(t *testing.T) {
	t.Parallel()

	for d, want := range map[time.Duration]string{4 * time.Second: "4.0", 2340 * time.Millisecond: "2.34", time.Millisecond: "0.001", 0: "0.0"} {
		out, err := orderedjson.Encode(orderedjson.Seconds(d), false)
		require.NoError(t, err)
		assert.Equal(t, want, string(out))
	}
	out, err := orderedjson.Encode(orderedjson.Int(-12), false)
	require.NoError(t, err)
	assert.Equal(t, "-12", string(out))
}
