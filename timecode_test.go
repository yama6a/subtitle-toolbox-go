package stbox_test

import (
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
)

// Ports testParseReadsEachStartTimeOfAFixtureAsItsParserDoes of tests/TimecodeTest.php of subtitle-toolbox-php for the
// formats that exist in Go. The YouTube chapters row joins with its format.
func TestParseTimeReadsEachStartTimeOfAFixtureAsItsParserDoes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		file   string
		format stbox.Format
		start  string
		rate   stbox.FrameRate
	}{
		{"SubRip", "srt/valid.srt", stbox.SubRip, `(?m)^(\S+) --> `, stbox.FrameRate{}},
		{"WebVTT", "vtt/missing_hours.vtt", stbox.WebVTT, `(?m)^(\S+) --> `, stbox.FrameRate{}},
		{"SBV", "sbv/from_srt.sbv", stbox.SBV, `(?m)^([\d:.]+),`, stbox.FrameRate{}},
		{"ASS", "ass/real/own_aegisub.ass", stbox.ASS, `(?m)^Dialogue: \d+,([^,]+),`, stbox.FrameRate{}},
		{"SubViewer", "subviewer/real/subviewer2_crlf.sub", stbox.SubViewer, `(?m)^([\d:.]+),[\d:.]+\r?$`, stbox.FrameRate{}},
		{"LRC", "lrc/one_digit_fraction.lrc", stbox.LRC, `(?m)^\[([\d:.]+)\]`, stbox.FrameRate{}},
		{"TTML", "ttml/real/bbc_ebu_tt_d.ttml", stbox.TTML, `<p [^>]*begin="([^"]+)"`, stbox.FrameRate{}},
		{"CSV, 25 fps", "csv/own_frame_times.csv", stbox.CSV, `(?m)^([\d:]+),`, stbox.FrameRate25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := stbox.ReadOptions{Format: tt.format}
			if !tt.rate.IsZero() {
				opts.CSV = csv.ReadOptions{FrameRate: tt.rate}
			}
			s, _, err := stbox.Load(testfiles.Path(t, tt.file), opts)
			require.NoError(t, err)
			raw, err := os.ReadFile(testfiles.Path(t, tt.file))
			require.NoError(t, err)

			var want []time.Duration
			for _, c := range s.All() {
				want = append(want, c.Start)
			}
			var got []time.Duration
			for _, m := range regexp.MustCompile(tt.start).FindAllStringSubmatch(string(raw), -1) {
				d, err := stbox.ParseTime(m[1], tt.rate)
				require.NoError(t, err, m[1])
				got = append(got, d)
			}
			require.NotEmpty(t, want)
			assert.Equal(t, want, got)
		})
	}
}
