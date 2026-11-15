package conformance_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
)

// TestCavena890Files loads each real file by its extension, because Cavena 890 has no signature. Each file writes back
// byte for byte, also when the cue format data is gone and the writer encodes the text again.
func TestCavena890Files(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(testfiles.Path(t, "cavena890/real/*.890"))
	require.NoError(t, err)
	require.Len(t, files, 7)
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(file)
			require.NoError(t, err)
			_, found := stbox.Detect(raw)
			assert.False(t, found)

			// A text encoding must not touch the bytes.
			for _, opts := range []stbox.ReadOptions{{}, {Encoding: "ISO-8859-7"}} {
				s, info, err := stbox.Load(file, opts)
				require.NoError(t, err)
				assert.Equal(t, stbox.Cavena890, info.Format)
				assert.Empty(t, info.SourceEncoding)
				assert.Empty(t, info.Warnings)
				assert.True(t, bytes.Equal(raw, writeAs(t, s, stbox.Cavena890)), "round trip with the stored blocks")

				for i, c := range s.All() {
					c.DeleteFormatData("cavena890")
					require.NoError(t, s.SetCue(i, c))
				}
				assert.True(t, bytes.Equal(raw, writeAs(t, s, stbox.Cavena890)), "round trip with encoded text")
			}

			_, _, err = parseBytes(raw, stbox.ReadOptions{})
			assert.ErrorIs(t, err, stbox.ErrUnknownFormat)
		})
	}
}

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

func TestMatroskaDVBSubtitlesReadLikeTheTransportStream(t *testing.T) {
	t.Parallel()

	for _, track := range [][2]int{{2, 257}, {3, 258}} {
		fromMKV, info, err := stbox.Load(testfiles.Path(t, "mkv/dvbsub.mkv"), stbox.ReadOptions{Track: track[0]})
		require.NoError(t, err)
		assert.Equal(t, stbox.PGS, info.Format)
		fromTS, _, err := stbox.Load(testfiles.Path(t, "mpegts/dvb_subtitles.ts"), stbox.ReadOptions{Track: track[1]})
		require.NoError(t, err)
		assert.Equal(t, fromTS.Cues(), fromMKV.Cues())
	}
}
