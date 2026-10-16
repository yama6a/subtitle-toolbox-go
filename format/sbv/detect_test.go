package sbv_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/sbv"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

// Rows of the SBV signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"SBV", "0:00:01.500,0:00:04.000\nHello\n", true},
		{"two hour digits", "00:00:01.500,00:00:04.000\nHello\n", true},
		{"spaces and tabs after the line", "0:00:01.500,0:00:04.000 \t\nHello\n", true},
		{"timing line only", "0:00:01.500,0:00:04.000", true},
		{"minutes of 60", "0:60:01.500,0:00:04.000\nHello\n", true},
		{"20 hour digits", "99999999999999999999:00:01.500,0:00:04.000\nHello\n", true},
		{"1 fraction digit", "0:00:01.5,0:00:04.000\nHello\n", false},
		{"SubRip timing line", "0:00:01,500 --> 0:00:04,000\nHello\n", false},
		{"SubViewer 2 timing line", "00:00:01.50,00:00:04.00\nHello\n", false},
		{"spaces around the comma", "0:00:01.500 , 0:00:04.000\nHello\n", false},
		{"text after the timing line", "0:00:01.500,0:00:04.000 x\nHello\n", false},
		{"timing line on the second line", "Hello\n0:00:01.500,0:00:04.000\n", false},
		{"SubRip", "1\n00:00:01,000 --> 00:00:04,000\nHello\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, sbv.Detect([]byte(tt.content)))
		})
	}
}

// Ports the SBV rows of testDetectsBrokenFixturesByTheirShape of tests/FormatDetectorTest.php of subtitle-toolbox-php.
func TestDetectBrokenFixtures(t *testing.T) {
	t.Parallel()

	assert.False(t, sbv.Detect(testfiles.Read(t, "sbv/missing_milli_digits.sbv")))
	assert.False(t, sbv.Detect(testfiles.Read(t, "sbv/srt_timestamps.sbv")))
	assert.True(t, sbv.Detect(testfiles.Read(t, "sbv/valid.sbv")))
}
