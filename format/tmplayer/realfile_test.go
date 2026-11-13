package tmplayer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports testRealFileSurvivesARoundTrip and testALineOutOfTimeOrderEndsTheCueBeforeIt of
// tests/Parsers/TmPlayerParserTest.php of subtitle-toolbox-php. The read goldens check what each file reads as,
// including the cue end that an out-of-order line sets.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file string
		// crlf is true when the file is the CR LF output of the writer, so a round trip is byte-identical.
		crlf bool
	}{
		{"tmplayer_crlf.txt", true},
		{"tmplayer_plus.txt", false},
		{"tmplayer_out_of_order.txt", false},
		{"tmplayer_multiline_bom.txt", false},
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
