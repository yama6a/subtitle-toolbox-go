package srt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// Ports testRealFileSurvivesARoundTrip of tests/Parsers/SubRipRealFilesTest.php of subtitle-toolbox-php. The read
// goldens check what each file reads as.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	files := []string{
		"swift_subtitle_kit_embedded_bom.srt", "own_styled.srt", "language_subtitles_dots_tester.srt",
		"language_subtitles_ssa_extensions.srt", "own_alignment_and_coordinates.srt", "own_cr_cr_lf.srt",
		"own_timestamp_without_millis.srt", "own_escaping.srt", "own_missing_empty_line.srt", "own_empty_cues.srt",
		"own_openai_whisper.srt", "own_whisper_cpp_diarize.srt", "own_ffmpeg.srt", "own_vtt_cue_settings.srt",
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, realDir+"/"+file)
			written := write(t, s, model.CommonWriteOptions{})
			back, _ := mustRead(t, written, false)
			assert.Equal(t, describe(s), describe(back))
			assert.Equal(t, written, write(t, back, model.CommonWriteOptions{}))
		})
	}
}

type described struct {
	Start, End  time.Duration
	Lines       []string
	Alignment   model.Alignment
	Coordinates *model.Data
}

// describe returns the cues as SubRip keeps them. The writer drops tags outside the SubRip set, such as <foo>.
func describe(s *model.Subtitle) []described {
	var out []described
	for _, c := range s.All() {
		var lines []string
		for _, line := range c.Lines {
			if line = markupcore.KeepTags(line, "b", "u", "i", "s", "font"); line != "" {
				lines = append(lines, line)
			}
		}
		coordinates, _ := c.FormatData("srt")
		out = append(out, described{c.Start, c.End, lines, c.Position.Alignment, coordinates})
	}
	return out
}

// Ports testOwnFileIsFormattedWithStandardTimestampsAndLeadingAlignment of tests/Parsers/SubRipRealFilesTest.php of
// subtitle-toolbox-php. The read golden holds the coordinates and the alignment.
func TestRealFileCoordinatesAndAlignment(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_alignment_and_coordinates.srt")
	assert.Equal(t, string(testfiles.Read(t, realDir+"/own_alignment_and_coordinates_formatted.srt")), write(t, s, model.CommonWriteOptions{}))
}

// Ports testOwnEscapingFileSurvivesARoundTripByteForByte and testTimestampWithoutMillisecondsIsWrittenWithMilliseconds
// of tests/Parsers/SubRipRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileOutput(t *testing.T) {
	t.Parallel()

	raw := string(testfiles.Read(t, realDir+"/own_escaping.srt"))
	s, _ := mustRead(t, raw, false)
	assert.Equal(t, raw, write(t, s, model.CommonWriteOptions{}))

	written := write(t, readFile(t, realDir+"/own_timestamp_without_millis.srt"), model.CommonWriteOptions{})
	assert.Contains(t, written, bom+"1\n00:01:39,000 --> 00:01:41,040\n(train brakes squeal)\n")
}

// Ports testCueWithoutTextIsKeptWithoutLines of tests/Parsers/SubRipRealFilesTest.php of subtitle-toolbox-php. The
// read golden holds the cues without lines, but not that their lines are nil. This test also checks that the writer keeps them.
func TestRealFileCueWithoutTextIsWritten(t *testing.T) {
	t.Parallel()

	empty := readFile(t, realDir+"/own_empty_cues.srt")
	for _, i := range []int{1, 3} {
		c, _ := empty.Cue(i)
		assert.Nil(t, c.Lines)
	}
	assert.Contains(t, write(t, empty, model.CommonWriteOptions{}), "\n\n2\n00:00:23,292 --> 00:00:28,898\n\n3\n")
}
