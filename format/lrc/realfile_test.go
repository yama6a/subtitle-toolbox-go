package lrc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports testRealFileSurvivesRoundTrip of tests/Parsers/LyricsParserTest.php of subtitle-toolbox-php. The read
// goldens check what each file reads as.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	files := []string{
		"justan-1.lrc", "justan-4.lrc", "lrc-maker-nami.lrc", "mantas-done-lrc.lrc", "own_whisper_cpp.lrc",
		"subsrt-sample.lrc", "handwritten-core.lrc", "handwritten-enhanced.lrc", "own-comment-before-earlier-line.lrc",
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, realDir+"/"+file)
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
		cues = append(cues, cueRow{c.Start.Round(10 * ms), c.End.Round(10 * ms), taggedText(c)})
	}
	return []any{cues, s.AllMetadata(), fileData(t, s), s.Comments()}
}
