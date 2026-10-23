package tmplayer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testRealFileParses, testRealFileSurvivesARoundTrip and testALineOutOfTimeOrderEndsTheCueBeforeIt of
// tests/Parsers/TmPlayerParserTest.php of subtitle-toolbox-php.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		cues        int
		first, last cueRow
		// crlf is true when the file is the CR LF output of the writer, so a round trip is byte-identical.
		crlf bool
	}{
		{
			"tmplayer_crlf.txt", 5,
			cueRow{time.Second, 5 * time.Second, []string{"The train to the coast", "leaves from platform two."}},
			cueRow{3600 * time.Second, 3605 * time.Second, []string{"We arrive on time today."}},
			true,
		},
		{
			"tmplayer_plus.txt", 4,
			cueRow{2 * time.Second, 6 * time.Second, []string{"Rain moves in from the west."}},
			cueRow{150 * time.Second, 155 * time.Second, []string{"Frost on the roads in the morning."}},
			false,
		},
		{
			"tmplayer_out_of_order.txt", 4,
			cueRow{time.Second, 4 * time.Second, []string{"The ferry leaves at nine."}},
			cueRow{12 * time.Second, 16 * time.Second, []string{"We arrive at the island at noon."}},
			false,
		},
		{
			"tmplayer_multiline_bom.txt", 3,
			cueRow{3 * time.Second, 7 * time.Second, []string{"The shop opens at eight.", "Fresh milk arrives at nine."}},
			cueRow{12 * time.Second, 16 * time.Second, []string{"Bring your own bag.", "Paper bags cost ten cents."}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, realDir+"/"+tt.file, model.CommonReadOptions{})
			require.Equal(t, tt.cues, s.Len())
			rows := cueRows(s)
			assert.Equal(t, tt.first, rows[0])
			assert.Equal(t, tt.last, rows[len(rows)-1])
		})
	}
	for _, tt := range tests {
		t.Run(tt.file+" round trip", func(t *testing.T) {
			t.Parallel()

			// The writer writes no end entry after the last cue, so the last cue of the written file lasts 4 s.
			common := model.CommonReadOptions{LastCueDuration: new(4 * time.Second)}
			s := readFile(t, realDir+"/"+tt.file, common)
			written := write(t, s, model.CommonWriteOptions{LineEnding: model.CRLF})
			back, warnings, err := read(written, common)
			require.NoError(t, err)
			require.Empty(t, warnings)
			assert.Equal(t, cueRows(s), cueRows(back))
			if tt.crlf {
				assert.Equal(t, string(testfiles.Read(t, realDir+"/"+tt.file)), written)
			}
		})
	}
}

func TestOutOfOrderFile(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []cueRow{
		{time.Second, 4 * time.Second, []string{"The ferry leaves at nine."}},
		{4 * time.Second, 7 * time.Second, []string{"Please mind the gap."}},
		{7 * time.Second, 10 * time.Second, []string{"Tickets are sold", "on the upper deck."}},
		{12 * time.Second, 16 * time.Second, []string{"We arrive at the island at noon."}},
	}, cueRows(readFile(t, realDir+"/tmplayer_out_of_order.txt", model.CommonReadOptions{})))
}
