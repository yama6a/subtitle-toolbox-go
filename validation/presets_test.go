package validation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v2"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
	"github.com/yama6a/subtitle-toolbox-go/v2/validation"
)

// Ports testNetflixEnglishPresetValues and testBbcPresetValues. Each preset is
// compared with a literal, so a changed number or a new rule fails here.
func TestPresetNumbers(t *testing.T) {
	t.Parallel()

	netflix, err := validation.NetflixEnglish(stbox.FrameRate{Num: 25, Den: 1})
	require.NoError(t, err)
	assert.Equal(t, validation.Rules{
		MaxCharactersPerSecond: 20,
		MaxCharactersPerLine:   42,
		MaxLinesPerCue:         2,
		MinDuration:            5 * time.Second / 6,
		MaxDuration:            7 * time.Second,
		MinGap:                 80 * time.Millisecond,
		NoOverlap:              true,
	}, netflix)
	assert.Equal(t, validation.Rules{MaxCharactersPerLine: 37, MaxWordsPerMinute: 180, MinDurationPerWord: 300 * time.Millisecond},
		validation.BBC())
	assert.Equal(t, validation.Rules{RequireCues: true, NoNegativeDuration: true, NoOverlap: true}, validation.Structure())
}

func TestNetflixEnglishGap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rate stbox.FrameRate
		want time.Duration
	}{
		{stbox.FrameRate{Num: 2997, Den: 125}, 83 * time.Millisecond},
		{stbox.FrameRate23976, 83 * time.Millisecond},
		{stbox.FrameRate{Num: 24, Den: 1}, 83 * time.Millisecond},
		{stbox.FrameRate{Num: 25, Den: 1}, 80 * time.Millisecond},
		{stbox.FrameRate{Num: 30000, Den: 1001}, 67 * time.Millisecond},
		{stbox.FrameRate{Num: 30000, Den: 1001, DropFrame: true}, 67 * time.Millisecond},
		{stbox.FrameRate{Num: 50, Den: 1}, 40 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.rate.String(), func(t *testing.T) {
			t.Parallel()

			rules, err := validation.NetflixEnglish(tt.rate)
			require.NoError(t, err)
			assert.Equal(t, tt.want, rules.MinGap.Round(time.Millisecond))
		})
	}
}

func TestNetflixEnglishErrors(t *testing.T) {
	t.Parallel()

	const zero = "The frame rate of the Netflix gap rule must be greater than 0."
	tests := []struct {
		name string
		rate stbox.FrameRate
		msg  string
	}{
		{"zero value", stbox.FrameRate{}, zero},
		{"zero numerator", stbox.FrameRate{Num: 0, Den: 1}, zero},
		{"negative", stbox.FrameRate{Num: -24, Den: 1}, zero},
		{
			"drop-frame at 25",
			stbox.FrameRate{Num: 25, Den: 1, DropFrame: true},
			"The frame rate must be 30000/1001, 2997/100, 60000/1001 or 2997/50 for drop-frame time codes, got 25.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := validation.NetflixEnglish(tt.rate)
			var arg *model.InvalidArgumentError
			require.ErrorAs(t, err, &arg)
			assert.Equal(t, &model.InvalidArgumentError{Option: "frameRate", Msg: tt.msg}, arg)
		})
	}
}

func TestStructureOnEmptySubtitle(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"-1 requireCues 0"}, lines(t, subtitle(), validation.Structure()))
}

// Ports testNetflixEnglishPresetOnOwnFile and testBbcPresetOnRealWebVttFile of tests/ValidationTest.php of
// subtitle-toolbox-php.
func TestPresetsOnFiles(t *testing.T) {
	t.Parallel()

	netflix, err := validation.NetflixEnglish(stbox.FrameRate{Num: 24, Den: 1})
	require.NoError(t, err)
	tests := []struct {
		name  string
		file  string
		rules validation.Rules
		want  []string
	}{
		{"netflix", "validation/own_netflix_checks.srt", netflix, []string{
			"1 maxCharactersPerLine 45, limit 42",
			"1 maxCharactersPerSecond 112.5, limit 20",
			"1 minDuration 0.4, limit 0.833333",
			"1 minGap 0.04, limit 0.083333",
			"2 maxCharactersPerLine 49, limit 42",
			"2 maxLinesPerCue 3, limit 2",
			"2 maxDuration 7.5, limit 7",
			"3 noOverlap 0.5",
		}},
		{"bbc", "vtt/real/w3c_voices.vtt", validation.BBC(), []string{
			"1 maxCharactersPerLine 55, limit 37",
			"1 maxWordsPerMinute 200, limit 180",
			"2 maxCharactersPerLine 39, limit 37",
			"2 maxWordsPerMinute 210, limit 180",
			"2 minSecondsPerWord 0.285714, limit 0.3",
			"6 maxWordsPerMinute 240, limit 180",
			"6 minSecondsPerWord 0.25, limit 0.3",
			"7 maxCharactersPerLine 61, limit 37",
			"7 maxWordsPerMinute 260, limit 180",
			"7 minSecondsPerWord 0.230769, limit 0.3",
			"8 maxWordsPerMinute 200, limit 180",
			"9 maxCharactersPerLine 47, limit 37",
			"9 maxWordsPerMinute 270, limit 180",
			"9 minSecondsPerWord 0.222222, limit 0.3",
			"10 maxCharactersPerLine 42, limit 37",
			"12 maxCharactersPerLine 58, limit 37",
			"12 maxWordsPerMinute 288, limit 180",
			"12 minSecondsPerWord 0.208333, limit 0.3",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, lines(t, load(t, tt.file), tt.rules))
		})
	}
}
