package srt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

type textCue struct {
	Start, End time.Duration
	Text       string
}

// Ports testRealFileParses and testRealFileSurvivesARoundTrip of tests/Parsers/SubRipRealFilesTest.php of subtitle-toolbox-php.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		cues        int
		first, last textCue
	}{
		{"swift_subtitle_kit_embedded_bom.srt", 3, textCue{0, 1500 * ms, "Synthetic fixture header"}, textCue{3100 * ms, 4500 * ms, "<i>Follow-up cue line.</i>"}},
		{"own_styled.srt", 10, textCue{17985 * ms, 20521 * ms, `<font color="#00ff00"><b>[train horn]</b></font>`}, textCue{720 * time.Second, 780 * time.Second, "<i>x</i>^3 * <i>x</i> = 100"}},
		{
			"language_subtitles_dots_tester.srt", 3,
			textCue{2501 * ms, 4500 * ms, "Tester for dots instead of commas in timestamps 1.0 by ale5000\n<b><i>Use VLC 1.0.3 or higher as reference</i></b>"},
			textCue{6501 * ms, 8500 * ms, "OK"},
		},
		{"language_subtitles_ssa_extensions.srt", 19, textCue{time.Second, 4 * time.Second, "Top-left: an7"}, textCue{97 * time.Second, 105 * time.Second, "Back to default"}},
		{"own_alignment_and_coordinates.srt", 10, textCue{time.Second, 3 * time.Second, "Plain first cue"}, textCue{19500 * ms, 21 * time.Second, `Keep this: {\some_unknown_tag} and {normal text}`}},
		//nolint:dupword // the text of the fixture
		{"own_cr_cr_lf.srt", 3, textCue{time.Second, 2500 * ms, "Every line in this file\nends with CR CR LF"}, textCue{5 * time.Second, 6500 * ms, "Last cue"}},
		{"own_timestamp_without_millis.srt", 6, textCue{99 * time.Second, 101040 * ms, "(train brakes squeal)"}, textCue{151400 * ms, 153440 * ms, "(radio playing\nsoft piano music)"}},
		{"own_escaping.srt", 5, textCue{time.Second, 3500 * ms, "I &lt;3 bread &amp; jam"}, textCue{12500 * ms, 15 * time.Second, `<font color="#ffcc00">Rain &amp; wind &gt;&gt; 40 km/h</font>`}},
		{"own_missing_empty_line.srt", 4, textCue{time.Second, 2500 * ms, "The bus to the airport is full."}, textCue{8 * time.Second, 10 * time.Second, "Mind the step."}},
		{"own_empty_cues.srt", 5, textCue{20105 * ms, 23292 * ms, "The ferry to the island leaves at noon."}, textCue{36100 * ms, 39 * time.Second, "The last boat comes back at six."}},
		{"own_openai_whisper.srt", 8, textCue{0, 4200 * ms, "Good evening and welcome to the timetable meeting."}, textCue{3603240 * ms, 3609 * time.Second, "Thank you all for coming tonight."}},
		{"own_whisper_cpp_diarize.srt", 6, textCue{0, 4120 * ms, "(speaker 0) Good morning, this is the station office."}, textCue{20100 * ms, 25880 * ms, "(speaker 0) That train now leaves at 6:50, ten minutes earlier."}},
		{"own_ffmpeg.srt", 7, textCue{500 * ms, 2750 * ms, "The bakery opens at six."}, textCue{62040 * ms, 65960 * ms, "Thank you for shopping with us"}},
		{"own_vtt_cue_settings.srt", 4, textCue{time.Second, 3500 * ms, "the tram to the old town"}, textCue{8200 * ms, 10 * time.Second, "every ten minutes"}},
	}
	for _, tt := range tests {
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
		out = append(out, described{c.Start, c.End, lines, c.Alignment, coordinates})
	}
	return out
}

// Ports testOwnFileKeepsCoordinatesAndAlignment and testOwnFileIsFormattedWithStandardTimestampsAndLeadingAlignment
// of tests/Parsers/SubRipRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileCoordinatesAndAlignment(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_alignment_and_coordinates.srt")
	second, _ := s.Cue(1)
	coordinates, _ := second.FormatData("srt")
	assert.Equal(t, model.NewData(model.Field{Key: "coordinates", Value: model.NewData(
		model.Field{Key: "x1", Value: int64(0)}, model.Field{Key: "x2", Value: int64(0)},
		model.Field{Key: "y1", Value: int64(50)}, model.Field{Key: "y2", Value: int64(100)},
	)}), coordinates)
	assert.Equal(t, []int{2, 2, 8, 4, 7, 8, 9, 2, 2, 2}, numpads(s))
	fourth, _ := s.Cue(3)
	assert.Equal(t, []string{"Middle left from the", "middle of a line, the second tag is ignored"}, fourth.Lines)
	eighth, _ := s.Cue(7)
	assert.Equal(t, []string{"<i>Italic</i> and <b>bold</b>", "<s>struck</s> and <u>underlined</u>"}, eighth.Lines)
	assert.Equal(t, string(testfiles.Read(t, realDir+"/own_alignment_and_coordinates_formatted.srt")), write(t, s, model.CommonWriteOptions{}))
}

// Ports testSsaExtensionsSetTheAlignment of tests/Parsers/SubRipRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileSSAExtensions(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/language_subtitles_ssa_extensions.srt")
	assert.Equal(t, []int{7, 8, 9, 4, 5, 6, 1, 2, 3, 1, 2, 3, 7, 8, 9, 4, 5, 6, 2}, numpads(s))
	c, _ := s.Cue(16)
	assert.Equal(t, []string{`[Deprecated] Middle-centre: \a10`}, c.Lines)
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

// Ports testAngleBracketTextThatIsNoTagStaysText of tests/Parsers/SubRipRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileAngleBrackets(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_angle_bracket_text.srt")
	var lines [][]string
	for _, c := range s.All() {
		lines = append(lines, c.Lines)
	}
	assert.Equal(t, [][]string{
		{"&lt;a sentence in angle brackets&gt;"},
		{`<i>Mind</i> the <font color="#ffcc00">gap</font> <foo>here</foo>`},
		{`<span class="exit">Exit</span> &lt;to the left&gt;`},
	}, lines)
}

// Ports testStrictModeStartsANewCueAtEveryTimingLine, testCueWithoutTextIsKeptWithoutLines and
// testWebVttCueSettingsAfterTheEndTimeSurviveAConversionToWebVtt of tests/Parsers/SubRipRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileBlocks(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []cueRow{
		{time.Second, 2500 * ms, []string{"The bus to the airport is full."}},
		{3 * time.Second, 5 * time.Second, []string{"The next one leaves at four."}},
		{5500 * ms, 7 * time.Second, []string{"Tickets are on sale inside."}},
		{8 * time.Second, 10 * time.Second, []string{"Mind the step."}},
	}, cueRows(readFile(t, realDir+"/own_missing_empty_line.srt")))

	empty := readFile(t, realDir+"/own_empty_cues.srt")
	rows := cueRows(empty)
	assert.Equal(t, []cueRow{{23292 * ms, 28898 * ms, nil}, {32 * time.Second, 36100 * ms, nil}}, []cueRow{rows[1], rows[3]})
	assert.Contains(t, write(t, empty, model.CommonWriteOptions{}), "\n\n2\n00:00:23,292 --> 00:00:28,898\n\n3\n")

	settings := readFile(t, realDir+"/own_vtt_cue_settings.srt")
	first, _ := settings.Cue(0)
	third, _ := settings.Cue(2)
	vtt, _ := first.FormatData("vtt")
	assert.Equal(t, model.NewData(model.Field{Key: "align", Value: "start"}, model.Field{Key: "position", Value: "0%"}), vtt)
	vtt, _ = third.FormatData("vtt")
	assert.Equal(t, model.NewData(model.Field{Key: "align", Value: "start"}), vtt)
}

func numpads(s *model.Subtitle) []int {
	var out []int
	for _, c := range s.All() {
		out = append(out, c.Alignment.Numpad())
	}
	return out
}
