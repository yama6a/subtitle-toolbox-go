package sbv_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var realFiles = []string{"subsrt_shape.sbv", "youtube_studio_lf.sbv", "youtube_studio_crlf.sbv"}

// Ports testRealFileSurvivesARoundTrip and testRealFileFormatsToItsOwnTextWithLfEndings of
// tests/Parsers/SbvRealFilesTest.php of subtitle-toolbox-php. The read goldens check what each file reads as.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	for _, file := range realFiles {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, realDir+"/"+file)

			written := write(t, s, model.CommonWriteOptions{})
			back, _ := mustRead(t, written, false)
			assert.Equal(t, cueRows(s), cueRows(back))
			assert.Equal(t, written, write(t, back, model.CommonWriteOptions{}))

			input := strings.ReplaceAll(string(testfiles.Read(t, realDir+"/"+file)), "\r\n", "\n")
			assert.Equal(t, strings.TrimRight(input, "\n")+"\n", written)
		})
	}
}

// Ports testCueWithoutTextIsKeptWithoutLines of tests/Parsers/SbvRealFilesTest.php of subtitle-toolbox-php. The read
// golden holds the cue without lines, but not that its lines are nil. This test also checks that the writer drops it.
func TestRealFileCueWithoutText(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_empty_cue.sbv")
	c, _ := s.Cue(1)
	assert.Nil(t, c.Lines)
	written := write(t, s, model.CommonWriteOptions{})
	back, _ := mustRead(t, written, false)
	assert.Equal(t, 2, back.Len())
}

func TestRealFileAngleBracketText(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_angle_bracket_text_from_srt.sbv")
	input := string(testfiles.Read(t, realDir+"/own_angle_bracket_text_from_srt.sbv"))
	assert.Equal(t, input, write(t, s, model.CommonWriteOptions{}))
}
