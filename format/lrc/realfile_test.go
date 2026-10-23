package lrc_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testRealFileParses and testRealFileSurvivesRoundTrip of tests/Parsers/LyricsParserTest.php of
// subtitle-toolbox-php. The CJK texts of justan and lrc-maker stay in testdata, so those rows check only the
// times. The read goldens check their text.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		cues        int
		first, last cueRow
	}{
		{"justan-1.lrc", 42, cueRow{0, time.Second, ""}, cueRow{202980 * ms, 207980 * ms, ""}},
		{"justan-4.lrc", 34, cueRow{0, 4 * time.Second, ""}, cueRow{202 * time.Second, 207 * time.Second, ""}},
		{"lrc-maker-nami.lrc", 39, cueRow{0, time.Second, ""}, cueRow{235536 * ms, 243353 * ms, ""}},
		{"mantas-done-lrc.lrc", 5, cueRow{8620 * ms, 9640 * ms, "Trains run early"}, cueRow{22630 * ms, 27630 * ms, "Rain comes later"}},
		{
			"own_whisper_cpp.lrc", 5,
			cueRow{0, 3880 * ms, "The train to the coast leaves at seven."},
			cueRow{60150 * ms, 65150 * ms, "Thank you for listening."},
		},
		{"subsrt-sample.lrc", 6, cueRow{12 * time.Second, 17200 * ms, "Line 1 about the train"}, cueRow{29020 * ms, 34020 * ms, "Line 6 about the bread"}},
		{
			"handwritten-core.lrc", 5,
			cueRow{5 * time.Second, 9400 * ms, "The train leaves at seven"},
			cueRow{32800 * ms, 37800 * ms, "Ring the bell, ring the bell"},
		},
		{
			"handwritten-enhanced.lrc", 3,
			cueRow{3 * time.Second, 5900 * ms, "<00:00:03.000>Slow <00:00:03.550>river <00:00:04.400>runs"},
			cueRow{9600 * ms, 14600 * ms, "<00:00:09.600>Into <00:00:10.100>the <00:00:10.650>sea"},
		},
		{"own-comment-before-earlier-line.lrc", 3, cueRow{time.Second, 5 * time.Second, "The door opens"}, cueRow{9 * time.Second, 14 * time.Second, "The shop closes"}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, realDir+"/"+tt.file)
			require.Equal(t, tt.cues, s.Len())
			rows := cueRows(s)
			first, last := rows[0], rows[len(rows)-1]
			if tt.first.Text == "" {
				first.Text, last.Text = "", ""
			}
			assert.Equal(t, tt.first, first)
			assert.Equal(t, tt.last, last)

			back, warnings, err := read(write(t, s, model.CommonWriteOptions{}), model.CommonReadOptions{})
			require.NoError(t, err)
			require.Empty(t, warnings)
			assert.Equal(t, describe(t, s), describe(t, back))
		})
	}
}

// describe returns the cues with times in whole centiseconds, the metadata, the file data and the comments,
// because the writer writes centiseconds.
func describe(t *testing.T, s *model.Subtitle) []any {
	t.Helper()

	var cues []cueRow
	for _, c := range s.All() {
		cues = append(cues, cueRow{c.Start.Round(10 * ms), c.End.Round(10 * ms), c.Text()})
	}
	return []any{cues, s.AllMetadata(), fileData(t, s), s.Comments()}
}
