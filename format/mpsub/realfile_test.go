package mpsub_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const phpNote = "NOTE=Created with the PHP Subtitle Toolbox (https://github.com/yama6a/subtitle-toolbox-php)\n"

func readFile(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, warnings, err := mpsub.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{}, mpsub.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	return s
}

// Ports testTimeBasedSampleFileParses, testTimeBasedSampleFileHeadersGoToMetadataAndFormatData and
// testTimeBasedSampleFileRoundTrips of tests/Parsers/MpSubParserTest.php of subtitle-toolbox-php.
func TestTimeBasedSampleFile(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/mplayer_doc_time.sub")
	rows := cueRows(s)
	require.Len(t, rows, 8)
	assert.Equal(t, cueRow{2 * time.Second, 5 * time.Second, []string{"The ferry leaves at nine."}}, rows[0])
	assert.Equal(t, cueRow{20250 * ms, 22 * time.Second, []string{"Yes, that one."}}, rows[5])
	assert.Equal(t, cueRow{27 * time.Second, 29 * time.Second, []string{"Let's go."}}, rows[7])
	assert.Equal(t, map[string]string{"title": "Harbour walk (sample)", "author": "Subtitle Toolbox"}, s.AllMetadata())
	assert.Equal(t, map[string]any{
		"FILE": "1048576,0123456789abcdef0123456789abcdef",
		"TYPE": "VIDEO",
		"NOTE": "Sample file written after the MPlayer format notes",
	}, fileData(s))

	output := write(t, s, model.CommonWriteOptions{}, mpsub.WriteOptions{})
	assert.True(t, strings.HasPrefix(output, "\xEF\xBB\xBFTITLE=Harbour walk (sample)\nAUTHOR=Subtitle Toolbox\n"))
	back, _ := mustRead(t, output, false)
	assert.Equal(t, rows, cueRows(back))
	assert.Equal(t, s.AllMetadata(), back.AllMetadata())
	assert.Equal(t, fileData(s), fileData(back))
	assert.Equal(t, output, write(t, back, model.CommonWriteOptions{}, mpsub.WriteOptions{}))
}

// Ports testFrameBasedSampleFileParses and testFrameBasedSampleFileRoundTripsWithinOneFrame of
// tests/Parsers/MpSubParserTest.php of subtitle-toolbox-php.
func TestFrameBasedSampleFile(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/mplayer_doc_frames.sub")
	assert.Equal(t, []cueRow{
		{2 * time.Second, 3500 * ms, []string{"Two seconds in,", "shown for one and a half seconds"}},
		{3540 * ms, 7540 * ms, []string{"One frame later,", "shown for four seconds"}},
		{8040 * ms, 11040 * ms, []string{"Half a second later,", "shown for three seconds"}},
	}, cueRows(s))
	assert.Equal(t, map[string]string{"title": "Harbour walk, frame based (sample)"}, s.AllMetadata())
	assert.Empty(t, fileData(s))

	output := write(t, s, model.CommonWriteOptions{}, mpsub.WriteOptions{FrameRate: 25})
	assert.Contains(t, output, "\nFORMAT=25\n")
	back, _ := mustRead(t, output, false)
	require.Equal(t, s.Len(), back.Len())
	for i, c := range s.All() {
		assert.InDelta(t, c.Start.Seconds(), back.Cues()[i].Start.Seconds(), 0.021)
		assert.InDelta(t, c.End.Seconds(), back.Cues()[i].End.Seconds(), 0.021)
		assert.Equal(t, c.Lines, back.Cues()[i].Lines)
	}
}

// Ports testFrameBasedFileParsesAndRoundTripsByteForByte of tests/Parsers/MpSubParserTest.php of subtitle-toolbox-php.
func TestFramesFileRoundTripsByteForByte(t *testing.T) {
	t.Parallel()

	raw := testfiles.Read(t, "mpsub/frames.mpsub")
	s := readFile(t, "mpsub/frames.mpsub")
	assert.Equal(t, []cueRow{{0, 2 * time.Second, []string{"Hello"}}, {3 * time.Second, 6 * time.Second, []string{"World"}}}, cueRows(s))
	assert.Equal(t, map[string]string{"title": "Big Buck Bunny", "author": "Jane Doe"}, s.AllMetadata())
	assert.Equal(t, map[string]any{"TYPE": "VIDEO"}, fileData(s))

	want := "\xEF\xBB\xBF" + strings.Replace(string(raw), "FORMAT=25\n", "FORMAT=25\n"+goNote, 1)
	assert.Equal(t, want, write(t, s, model.CommonWriteOptions{}, mpsub.WriteOptions{FrameRate: 25}))
}

// Ports testFormatterOutputParsesBackToTheSameCues of tests/Parsers/MpSubParserTest.php of subtitle-toolbox-php.
func TestExpectedWriterOutputReadsBack(t *testing.T) {
	t.Parallel()

	raw := testfiles.Read(t, "mpsub/valid.mpsub")
	s := readFile(t, "mpsub/valid.mpsub")
	rows := cueRows(s)
	require.Len(t, rows, 6)
	assert.Equal(t, cueRow{0, 456 * ms, []string{"Only milliseconds"}}, rows[0])
	assert.Equal(t, 3599999999*ms, rows[5].End)
	assert.Empty(t, s.AllMetadata())
	assert.Equal(t, string(raw), write(t, s, model.CommonWriteOptions{}, mpsub.WriteOptions{}))
}

// Ports testSubtitleIsFormattedCorrectlyAndXmlTagsAreStripped and testFrameRateOptionWritesFrames of
// tests/Formatters/MpSubFormatterTest.php of subtitle-toolbox-php. The expected files hold the PHP NOTE line,
// which the Go default replaces.
func TestWriteSubRipAsMPSub(t *testing.T) {
	t.Parallel()

	s, warnings, err := srt.Read(bytes.NewReader(testfiles.Read(t, "srt/valid.srt")), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)

	tests := []struct {
		file string
		opts mpsub.WriteOptions
	}{
		{"mpsub/valid.mpsub", mpsub.WriteOptions{}},
		{"mpsub/valid_25fps.mpsub", mpsub.WriteOptions{FrameRate: 25}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			want := strings.Replace(string(testfiles.Read(t, tt.file)), phpNote, goNote, 1)
			require.NotContains(t, want, "PHP")
			assert.Equal(t, want, write(t, s, model.CommonWriteOptions{}, tt.opts))
		})
	}
}

// Ports testEmptyCuesAndLinesAreSkippedSoTheOutputReadsBack of tests/Formatters/MpSubFormatterTest.php of
// subtitle-toolbox-php. The third PHP file, validation/own_netflix_checks.srt, is not in this repository, so its
// cue of empty tags is built here.
func TestEmptyCuesAndLinesAreSkippedSoTheOutputReadsBack(t *testing.T) {
	t.Parallel()

	sources := map[string]string{
		"empty cues":        string(testfiles.Read(t, "srt/real/own_empty_cues.srt")),
		"empty middle line": string(testfiles.Read(t, "srt/real/own_alignment_and_coordinates.srt")),
		"only empty tags":   "1\n00:00:01,000 --> 00:00:03,000\nBonjour\n\n2\n00:00:04,000 --> 00:00:05,000\n<i></i>\n\n3\n00:00:06,000 --> 00:00:07,000\nSalut\n",
	}
	for name, input := range sources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s, _, err := srt.Read(strings.NewReader(input), model.CommonReadOptions{Lenient: true}, srt.ReadOptions{})
			require.NoError(t, err)
			var want []cueRow
			for _, c := range s.All() {
				if lines := markupcore.PlainLines(c.Lines); len(lines) > 0 {
					want = append(want, cueRow{c.Start, c.End, lines})
				}
			}
			require.NotEmpty(t, want)
			back, _ := mustRead(t, write(t, s, model.CommonWriteOptions{}, mpsub.WriteOptions{}), false)
			require.Equal(t, len(want), back.Len())
			for i, c := range back.Cues() {
				assert.InDelta(t, float64(want[i].Start), float64(c.Start), float64(ms), "start %d", i)
				assert.InDelta(t, float64(want[i].End), float64(c.End), float64(ms), "end %d", i)
				assert.Equal(t, want[i].Lines, c.Lines)
			}
		})
	}
}
