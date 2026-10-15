package cli

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSpec() []option {
	return []option{
		flagOption("force", "Overwrite."),
		valueOption("by", "SECONDS", "Seconds."),
		{name: "output", short: "o", value: "PATH", help: "Output."},
		{name: "brackets", value: "PAIR", help: "Pairs.", repeatable: true},
	}
}

func parse(t *testing.T, words ...string) *args {
	t.Helper()

	a, err := parseArgs(words, testSpec())
	require.NoError(t, err)
	return a
}

func TestParseArgsReadsOptionFormsAndPositionals(t *testing.T) {
	t.Parallel()

	a := parse(t, "a.srt", "--by", "-2.5", "-", "--force", "-o", "out.srt", "--brackets={}", "--brackets", "**")
	assert.Equal(t, []string{"a.srt", "-"}, a.positionals)
	assert.True(t, a.Has("force"))
	by, ok, err := a.Float("by")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.InDelta(t, -2.5, by, 0)
	out, ok := a.Value("output")
	assert.True(t, ok)
	assert.Equal(t, "out.srt", out)
	assert.Equal(t, []string{"{}", "**"}, a.Values("brackets"))
	_, ok = a.Value("missing")
	assert.False(t, ok)
	assert.Empty(t, a.Values("missing"))
	assert.False(t, a.Has("missing"))
}

func TestParseArgsSplitsAtTheFirstEqualsSignAndStopsAtDoubleDash(t *testing.T) {
	t.Parallel()

	a := parse(t, "--by=-1", "--output=a=b", "--force", "--force", "--", "--force", "-o")
	by, _ := a.Value("by")
	assert.Equal(t, "-1", by)
	out, _ := a.Value("output")
	assert.Equal(t, "a=b", out)
	assert.Equal(t, []string{"--force", "-o"}, a.positionals)
	assert.True(t, a.Has("force"))

	a = parse(t, "--", "--force")
	assert.False(t, a.Has("force"))
	assert.Equal(t, []string{"--force"}, a.positionals)

	a = parse(t, "--by", "")
	by, ok := a.Value("by")
	assert.True(t, ok)
	assert.Empty(t, by)
}

func TestParseArgsFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		want  string
	}{
		{"unknown long option", []string{"--nope"}, "Unknown option --nope."},
		{"unknown long option with a value", []string{"--nope=1"}, "Unknown option --nope."},
		{"unknown short option", []string{"-x"}, "Unknown option -x."},
		{"negative number", []string{"-2.5"}, "Unknown option -2.5."},
		{"short option with an attached value", []string{"-ofile"}, "Unknown option -ofile."},
		{"short option with equals sign", []string{"-o=file"}, "Unknown option -o=file."},
		{"long name with one dash", []string{"-force"}, "Unknown option -force."},
		{"flag with a value", []string{"--force=yes"}, "The option --force takes no value."},
		{"flag with an empty value", []string{"--force="}, "The option --force takes no value."},
		{"missing value", []string{"--by"}, "The option --by needs a value."},
		{"missing value of a short option", []string{"a.srt", "-o"}, "The option --output needs a value."},
		{"value given twice", []string{"--by", "1", "--by", "2"}, "The option --by is given twice."},
		{"value given twice by its short name", []string{"--output", "a", "-o", "b"}, "The option --output is given twice."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseArgs(tt.words, testSpec())
			var ue *usageError
			require.ErrorAs(t, err, &ue)
			assert.Equal(t, tt.want, ue.msg)
		})
	}
}

func TestNumberRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in     string
		want   float64
		wantOK bool
	}{
		{"2.5", 2.5, true},
		{"-2.5", -2.5, true},
		{"+1", 1, true},
		{".5", 0.5, true},
		{"5.", 5, true},
		{"1e3", 1000, true},
		{"1E-3", 0.001, true},
		{" 2 ", 2, true},
		{"\t\n\r\v\f7\n", 7, true},
		{"1e999", math.Inf(1), true},
		{"0x1A", 0, false},
		{"1_000", 0, false},
		{"inf", 0, false},
		{"NaN", 0, false},
		{"", 0, false},
		{".", 0, false},
		{"1e", 0, false},
		{"--1", 0, false},
		{"1 2", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, ok := parseNumber(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got) //nolint:testifylint // exact values
		})
	}
}

func TestNumberGetters(t *testing.T) {
	t.Parallel()

	a := parse(t, "--by", "3", "--output", "0")
	f, ok, err := a.PositiveFloat("by")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.InDelta(t, 3.0, f, 0)
	n, ok, err := a.PositiveInt("by")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, int64(3), n)
	n, _, err = a.Int("output", 0)
	require.NoError(t, err)
	assert.Zero(t, n)
	n, _, err = a.IntRange("by", 1, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
	f, _, err = a.NonNegativeFloat("output")
	require.NoError(t, err)
	assert.Zero(t, f)

	for _, getter := range []func(string) (float64, bool, error){a.Float, a.PositiveFloat, a.NonNegativeFloat} {
		_, ok, err := getter("missing")
		require.NoError(t, err)
		assert.False(t, ok)
	}
	for _, getter := range []func(string) (int64, bool, error){a.PositiveInt, func(n string) (int64, bool, error) { return a.Int(n, 0) }} {
		_, ok, err := getter("missing")
		require.NoError(t, err)
		assert.False(t, ok)
	}
}

func TestTimeGetters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want time.Duration
	}{
		{"2.5", 2500 * time.Millisecond},
		{"-2.5", -2500 * time.Millisecond},
		{"-00:00:02,500", -2500 * time.Millisecond},
		{"01:02:03.456", time.Hour + 2*time.Minute + 3456*time.Millisecond},
		{"01:02.5", 62500 * time.Millisecond},
		{"00:00:01.2", 1200 * time.Millisecond},
		{"00:00", 0},
		{"359999999", 359999999 * time.Second},
		{"359999999.9994", 359999999999 * time.Millisecond},
		{"1.0005", 1001 * time.Millisecond},
		{"-1.0005", -1001 * time.Millisecond},
		{"0.0004", 0},
		{" 2 ", 2 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, ok, err := parse(t, "--by", tt.in).Time("by")
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}

	d, _, err := parse(t, "--by=00:00:01.2").PositiveTime("by")
	require.NoError(t, err)
	assert.Equal(t, 1200*time.Millisecond, d)
	d, _, err = parse(t, "--by", "00:00").NonNegativeTime("by")
	require.NoError(t, err)
	assert.Zero(t, d)
	_, ok, err := parse(t).Time("by")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestChoiceIgnoresASCIICaseAndReturnsTheAllowedSpelling(t *testing.T) {
	t.Parallel()

	a := parse(t, "--by", "Top-BOTTOM")
	got, ok, err := a.Choice("by", "stack", "top-bottom")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "top-bottom", got)
	_, ok, err = a.Choice("missing", "stack")
	require.NoError(t, err)
	assert.False(t, ok)

	// U+212A KELVIN SIGN folds to k in strings.EqualFold, but not in PHP strcasecmp.
	_, _, err = parse(t, "--by", "K").Choice("by", "k")
	require.Error(t, err)
}

func TestGetterFailures(t *testing.T) {
	t.Parallel()

	floatGetter := func(get func(*args) func(string) (float64, bool, error)) func(*args) error {
		return func(a *args) error { _, _, err := get(a)("by"); return err }
	}
	timeGetter := func(get func(*args) func(string) (time.Duration, bool, error)) func(*args) error {
		return func(a *args) error { _, _, err := get(a)("by"); return err }
	}
	var (
		float            = floatGetter(func(a *args) func(string) (float64, bool, error) { return a.Float })
		positiveFloat    = floatGetter(func(a *args) func(string) (float64, bool, error) { return a.PositiveFloat })
		nonNegativeFloat = floatGetter(func(a *args) func(string) (float64, bool, error) { return a.NonNegativeFloat })
		seconds          = timeGetter(func(a *args) func(string) (time.Duration, bool, error) { return a.Time })
		positiveSeconds  = timeGetter(func(a *args) func(string) (time.Duration, bool, error) { return a.PositiveTime })
		nonNegSeconds    = timeGetter(func(a *args) func(string) (time.Duration, bool, error) { return a.NonNegativeTime })
		positiveInt      = func(a *args) error { _, _, err := a.PositiveInt("by"); return err }
		intOf            = func(lo int64) func(*args) error {
			return func(a *args) error { _, _, err := a.Int("by", lo); return err }
		}
		intRange = func(lo, hi int64) func(*args) error {
			return func(a *args) error { _, _, err := a.IntRange("by", lo, hi); return err }
		}
		choice = func(allowed ...string) func(*args) error {
			return func(a *args) error { _, _, err := a.Choice("by", allowed...); return err }
		}
	)
	timecodeMessage := func(v string) string {
		return `The option --by needs seconds or a timecode such as 00:01:02.500, got "` + v + `".`
	}
	tests := []struct {
		name   string
		getter func(*args) error
		value  string
		want   string
	}{
		{"not a number", float, "abc", `The option --by needs a number, got "abc".`},
		{"quote in the value", float, `a"b`, `The option --by needs a number, got "a"b".`},
		{"infinite", float, "1e999", `The option --by needs a finite number, got "1e999".`},
		{"negative infinite", float, "-1e999", `The option --by needs a finite number, got "-1e999".`},
		{"infinite positive", positiveFloat, "1e999", `The option --by needs a finite number, got "1e999".`},
		{"zero", positiveFloat, "0", "The option --by must be greater than 0."},
		{"fraction for int", positiveInt, "1.5", `The option --by needs a whole number of 1 or more, got "1.5".`},
		{"zero for int", positiveInt, "0", `The option --by needs a whole number of 1 or more, got "0".`},
		{"int overflow", positiveInt, "99999999999999999999", `The option --by needs a whole number of 1 or more, got "99999999999999999999".`},
		{"negative", nonNegativeFloat, "-0.5", "The option --by must not be negative."},
		{"infinite non-negative", nonNegativeFloat, "1e999", `The option --by needs a finite number, got "1e999".`},
		{"five fields", seconds, "1:2:3:4:5", timecodeMessage("1:2:3:4:5")},
		{"frames", seconds, "00:00:01:12", timecodeMessage("00:00:01:12")},
		{"lone minus", seconds, "-", timecodeMessage("-")},
		{"two minus signs", seconds, "--00:01", timecodeMessage("--00:01")},
		{"timecode of 100000 hours", seconds, "100000:00:00", timecodeMessage("100000:00:00")},
		{"infinite seconds", seconds, "1e999", `The option --by needs a finite number, got "1e999".`},
		{"huge seconds", seconds, "1e300", `The option --by needs a time below 100000 hours, got "1e300".`},
		{"rounds to 100000 hours", seconds, "359999999.9996", `The option --by needs a time below 100000 hours, got "359999999.9996".`},
		{"rounds to -100000 hours", seconds, "-359999999.9996", `The option --by needs a time below 100000 hours, got "-359999999.9996".`},
		{"time code of 5124096 hours", seconds, "5124096:00:00", timecodeMessage("5124096:00:00")},
		{"100000 hours", seconds, "360000000", `The option --by needs a time below 100000 hours, got "360000000".`},
		{"-100000 hours", seconds, "-360000000", `The option --by needs a time below 100000 hours, got "-360000000".`},
		{"zero timecode", positiveSeconds, "00:00", "The option --by must be greater than 0."},
		{"negative timecode", nonNegSeconds, "-00:00:00.5", "The option --by must not be negative."},
		{"fraction", intRange(0, 9), "1.5", `The option --by needs a whole number from 0 to 9, got "1.5".`},
		{"below min", intRange(1, 9), "0", `The option --by needs a whole number from 1 to 9, got "0".`},
		{"above max", intRange(1, 9), "10", `The option --by needs a whole number from 1 to 9, got "10".`},
		{"negative whole number", intOf(0), "-1", `The option --by needs a whole number of 0 or more, got "-1".`},
		{"too large", intOf(0), "99999999999999999999", `The option --by needs a whole number of 0 or more, got "99999999999999999999".`},
		{"19 digits", intOf(0), "1000000000000000000", `The option --by needs a whole number of 0 or more, got "1000000000000000000".`},
		{"plus sign", intOf(0), "+1", `The option --by needs a whole number of 0 or more, got "+1".`},
		{"empty", intOf(0), "", `The option --by needs a whole number of 0 or more, got "".`},
		{"exponent", intOf(0), "1e3", `The option --by needs a whole number of 0 or more, got "1e3".`},
		{"one choice", choice("lf"), "cr", `The option --by must be lf, got "cr".`},
		{"two choices", choice("lf", "crlf"), "cr", `The option --by must be lf or crlf, got "cr".`},
		{"three choices", choice("k", "kf", "ko"), "cr", `The option --by must be k, kf or ko, got "cr".`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.getter(parse(t, "--by", tt.value))
			var ue *usageError
			require.ErrorAs(t, err, &ue)
			assert.Equal(t, tt.want, ue.msg)
		})
	}
}

func TestWholeNumberKeepsLeadingZerosOutOfTheDigitLimit(t *testing.T) {
	t.Parallel()

	n, ok, err := parse(t, "--by", "000000000000000000000999999999999999999").Int("by", 0)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, int64(999999999999999999), n)
}
