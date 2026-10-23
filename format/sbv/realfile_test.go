package sbv_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

type textCue struct {
	Start, End time.Duration
	Text       string
}

var realFiles = []struct {
	file        string
	cues        int
	first, last textCue
}{
	{
		"subsrt_shape.sbv", 5,
		textCue{480 * ms, 3950 * ms, "&gt;&gt; MARA: Good morning, this is Mara Lind[br]and this is Tom Berg"},
		textCue{16300 * ms, 21150 * ms, "First, we put the flour and the water on the table"},
	},
	{
		"youtube_studio_lf.sbv", 9,
		textCue{0, 2240 * ms, "[Music]"},
		textCue{time.Hour, time.Hour + 3500*ms, "That was the weather. Thank you for watching.\n[Applause]"},
	},
	{
		"youtube_studio_crlf.sbv", 6,
		textCue{1500 * ms, 4 * time.Second, "The 7:15 train to the harbour leaves from platform 2."},
		textCue{18430 * ms, 21 * time.Second, "The next stop is Market Square."},
	},
}

// Ports testRealFileParses, testRealFileSurvivesARoundTrip and testRealFileFormatsToItsOwnTextWithLfEndings
// of tests/Parsers/SbvRealFilesTest.php of subtitle-toolbox-php.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	for _, tt := range realFiles {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, realDir+"/"+tt.file)
			require.Equal(t, tt.cues, s.Len())
			first, _ := s.Cue(0)
			last, _ := s.Cue(s.Len() - 1)
			gotFirst := textCue{first.Start, first.End, first.Text()}
			gotLast := textCue{last.Start, last.End, last.Text()}
			assert.Equal(t, tt.first, gotFirst)
			assert.Equal(t, tt.last, gotLast)

			written := write(t, s, model.CommonWriteOptions{})
			back, _ := mustRead(t, written, false)
			assert.Equal(t, cueRows(s), cueRows(back))
			assert.Equal(t, written, write(t, back, model.CommonWriteOptions{}))

			input := strings.ReplaceAll(string(testfiles.Read(t, realDir+"/"+tt.file)), "\r\n", "\n")
			assert.Equal(t, strings.TrimRight(input, "\n")+"\n", written)
		})
	}
}

// Ports testCueWithoutTextIsKeptWithoutLines of tests/Parsers/SbvRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileCueWithoutText(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_empty_cue.sbv")
	require.Equal(t, 3, s.Len())
	c, _ := s.Cue(1)
	assert.Equal(t, cueRow{23292 * ms, 28898 * ms, nil}, cueRow{c.Start, c.End, c.Lines})

	written := write(t, s, model.CommonWriteOptions{})
	back, _ := mustRead(t, written, false)
	assert.Equal(t, 2, back.Len())
}

func TestRealFileAngleBracketText(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_angle_bracket_text_from_srt.sbv")
	for _, c := range s.All() {
		for _, line := range c.Lines {
			assert.NotContains(t, line, "<")
			assert.NotContains(t, line, ">")
		}
	}
	input := string(testfiles.Read(t, realDir+"/own_angle_bracket_text_from_srt.sbv"))
	assert.Equal(t, input, write(t, s, model.CommonWriteOptions{}))
}
