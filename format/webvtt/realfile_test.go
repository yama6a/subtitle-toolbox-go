package webvtt_test

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

type cueRow struct {
	Start, End time.Duration
	Text       string
}

func row(c model.Cue) cueRow {
	return cueRow{Start: c.Start, End: c.End, Text: c.Text()}
}

func rows(s *model.Subtitle) []cueRow {
	var out []cueRow
	for _, c := range s.All() {
		out = append(out, row(c))
	}
	return out
}

// Ports testRealFileParses of tests/Parsers/WebVttRealFileTest.php of subtitle-toolbox-php.
func TestReadRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		cues        int
		first, last cueRow
	}{
		{"astisub_carriage_return.vtt", 3, cueRow{0, 3766 * ms, "The bakery on the corner opens\nat six o'clock every morning."}, cueRow{10733 * ms, 14066 * ms, "and sells the last one\nbefore the noon train."}},
		{"astisub_html_entities.vtt", 3, cueRow{331 * ms, 3750 * ms, "The morning train left the station on time, \u00a0\n&amp; the rain stopped at noon."}, cueRow{6331 * ms, 9675 * ms, "the temperature is &lt; ten degrees today."}},
		{"mantas_styles.vtt", 1, cueRow{0, 10 * sec, "Hello world."}, cueRow{0, 10 * sec, "Hello world."}},
		{"w3c_chapters.vtt", 4, cueRow{0, 10700 * ms, "Title Slide"}, cueRow{110100 * ms, 213 * sec, "Requirements of a Video text format"}},
		{"w3c_cue_settings.vtt", 3, cueRow{0, 4 * sec, "Where did he go?"}, cueRow{4 * sec, 6500 * ms, "What are you waiting for?"}},
		{"w3c_regions.vtt", 6, cueRow{0, 20 * sec, "<v Fred>Hi, my name is Fred"}, cueRow{12500 * ms, 32500 * ms, "<v Fred>OK, let's go."}},
		{"w3c_styles.vtt", 1, cueRow{0, 10 * sec, "Hello <b>world</b>."}, cueRow{0, 10 * sec, "Hello <b>world</b>."}},
		{"w3c_timestamps.vtt", 3, cueRow{0, 8 * sec, "<c>No match (no timestamps)</c>"}, cueRow{16 * sec, 24 * sec, "<00:00:16.000> <c>This</c>\n<00:00:18.000> <c>can</c>\n<00:00:20.000> <c>match</c>\n<00:00:22.000> <c>:past/:future</c>\n<00:00:24.000>"}},
		{"w3c_voices.vtt", 13, cueRow{11 * sec, 13 * sec, "<v Anna Berg>We are at the train station"}, cueRow{35500 * ms, 38 * sec, "<v Anna Berg>You know the rain is so heavy my umbrella is leaking here."}},
		{"webvttpy_netflix.vtt", 30, cueRow{7960 * ms, 9480 * ms, "[Rosa] <i>En 1928,</i>"}, cueRow{107760 * ms, 108800 * ms, "R\u00e1pido."}},
		{"own_empty_cues.vtt", 5, cueRow{20105 * ms, 23292 * ms, "The ferry to the island leaves at noon."}, cueRow{36100 * ms, 39 * sec, "The last boat comes back at six."}},
		{"own_hour_digits.vtt", 7, cueRow{800 * ms, 2933 * ms, "The first train leaves at six."}, cueRow{3600022860 * ms, 3600025560 * ms, "The station closes for the night."}},
		{"own_settings_without_space.vtt", 2, cueRow{0, sec, "The gate opens at eight."}, cueRow{2 * sec, 3500 * ms, "Boarding starts at half past."}},
		{"own_ytdlp_auto_captions.vtt", 2, cueRow{0, 2310 * ms, "the<00:00:00.480><c> ferry</c><00:00:00.960><c> leaves</c>"}, cueRow{2310 * ms, 5 * sec, "at<00:00:02.800><c> noon</c>"}},
		{"own_openai_whisper.vtt", 6, cueRow{0, 3600 * ms, "This is the second part of the timetable meeting."}, cueRow{3601500 * ms, 3606880 * ms, "Yes, the timetable starts on the first of December."}},
		{"own_openai_whisper_highlight_words.vtt", 11, cueRow{0, 320 * ms, "<u>The</u> next train leaves at nine."}, cueRow{3800 * ms, 4400 * ms, "Platform four is <u>closed.</u>"}},
		{"own_ffmpeg.vtt", 7, cueRow{500 * ms, 2750 * ms, "The bakery opens at six."}, cueRow{62040 * ms, 65960 * ms, "Thank you for shopping with us"}},
		{"webvttpy_youtube.vtt", 4, cueRow{286070 * ms, 286470 * ms, "okay"}, cueRow{305069 * ms, 305400 * ms, "the train<c.colorE5E5E5> leaves</c><c.colorCCCCCC> at ten today\n</c>"}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, filepath.Join(realDir, tt.file))
			got := rows(s)
			require.Len(t, got, tt.cues)
			assert.Equal(t, tt.first, got[0])
			assert.Equal(t, tt.last, got[len(got)-1])
		})
	}
}

// Ports testRealFileRoundTripKeepsAllData of tests/Parsers/WebVttRealFileTest.php of subtitle-toolbox-php.
func TestRealFilesRoundTripKeepsAllData(t *testing.T) {
	t.Parallel()

	for _, path := range realFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			s := readFile(t, path)
			var out bytes.Buffer
			require.NoError(t, webvtt.Write(&out, s, model.CommonWriteOptions{}, webvtt.WriteOptions{}))
			again, _ := read(t, out.Bytes(), false)
			assert.Equal(t, describe(t, s), describe(t, again))
			assert.Equal(t, s.Comments(), again.Comments())
			d, _ := s.FormatData("vtt")
			dAgain, _ := again.FormatData("vtt")
			assert.Equal(t, d, dAgain)

			var second bytes.Buffer
			require.NoError(t, webvtt.Write(&second, again, model.CommonWriteOptions{}, webvtt.WriteOptions{}))
			assert.Equal(t, out.String(), second.String())
		})
	}
}

// describe returns what a round trip keeps of each cue. Cue numbers fill in missing identifiers.
func describe(t *testing.T, s *model.Subtitle) []any {
	t.Helper()

	var out []any
	for _, c := range s.All() {
		out = append(out, []any{c.Start, c.End, c.Lines, c.Alignment, cueData(t, c)})
	}
	return out
}

// Ports the single-file tests of tests/Parsers/WebVttRealFileTest.php of subtitle-toolbox-php.
func TestReadRealFileDetails(t *testing.T) {
	t.Parallel()

	t.Run("regions and cue settings", func(t *testing.T) {
		t.Parallel()

		s := readFile(t, filepath.Join(realDir, "w3c_regions.vtt"))
		d, _ := s.FormatData("vtt")
		regions, _ := d.List("regions")
		ids := make([]string, 0, len(regions))
		for _, r := range regions {
			id, _ := r.(*model.Data).String("id")
			ids = append(ids, id)
		}
		assert.Equal(t, []string{"fred", "bill"}, ids)
		c, _ := s.Cue(1)
		assert.Equal(t, vttData(model.Field{Key: "region", Value: "bill"}, model.Field{Key: "align", Value: "right"}, model.Field{Key: "alignment", Value: int64(3)}), cueData(t, c))
		assert.Equal(t, model.BottomRight, c.Alignment)
	})
	t.Run("a cue without text has no lines", func(t *testing.T) {
		t.Parallel()

		s := readFile(t, filepath.Join(realDir, "own_empty_cues.vtt"))
		c, _ := s.Cue(1)
		assert.Equal(t, "2", c.ID)
		assert.Equal(t, cueRow{23292 * ms, 28898 * ms, ""}, row(c))
		assert.Nil(t, c.Lines)
		c, _ = s.Cue(3)
		assert.Equal(t, cueRow{32 * sec, 36100 * ms, ""}, row(c))
		assert.Equal(t, vttData(model.Field{Key: "align", Value: "middle"}, model.Field{Key: "line", Value: "90%"}), cueData(t, c))
	})
	t.Run("hours of any number of digits", func(t *testing.T) {
		t.Parallel()

		s := readFile(t, filepath.Join(realDir, "own_hour_digits.vtt"))
		var starts []time.Duration
		for _, c := range s.All() {
			starts = append(starts, c.Start)
		}
		assert.Equal(t, []time.Duration{800 * ms, 36010940 * ms, 36030940 * ms, 36040940 * ms, 360010750 * ms, 360030750 * ms, 3600022860 * ms}, starts)
		out := write(t, s, model.CommonWriteOptions{})
		assert.Contains(t, out, "\n10:00:30.940 --> 10:00:40.750\n")
		assert.Contains(t, out, "\n00:00:00.800 --> 00:00:02.933\n")
	})
	t.Run("settings right after the end time", func(t *testing.T) {
		t.Parallel()

		s := readFile(t, filepath.Join(realDir, "own_settings_without_space.vtt"))
		c, _ := s.Cue(0)
		assert.Equal(t, vttData(model.Field{Key: "line", Value: "40%"}, model.Field{Key: "size", Value: "40%"}), cueData(t, c))
		c, _ = s.Cue(1)
		assert.Equal(t, vttData(model.Field{Key: "align", Value: "start"}), cueData(t, c))
		assert.Contains(t, write(t, s, model.CommonWriteOptions{}), "\n00:00:00.000 --> 00:00:01.000 line:40% size:40%\n")
	})
	t.Run("YouTube header lines", func(t *testing.T) {
		t.Parallel()

		s := readFile(t, filepath.Join(realDir, "webvttpy_youtube.vtt"))
		d, _ := s.FormatData("vtt")
		lines, _ := d.Strings("headerLines")
		assert.Equal(t, []string{"Kind: captions", "Language: en", "Style:"}, lines[:3])
		assert.Equal(t, "##", lines[len(lines)-1])
	})
	t.Run("comments and header text", func(t *testing.T) {
		t.Parallel()

		s := readFile(t, filepath.Join(realDir, "webvttpy_comments.vtt"))
		d, _ := s.FormatData("vtt")
		assert.Equal(t, vttData(model.Field{Key: "header", Value: "- Translation of a weather report"}), d)
		before := make([]int, 0, len(s.Comments()))
		for _, c := range s.Comments() {
			before = append(before, c.BeforeCue)
		}
		assert.Equal(t, []int{0, 2, 3}, before)
		assert.Equal(t, "end of file", s.Comments()[2].Text)
	})
}

// Ports the rolling cue tests of tests/Parsers/WebVttRealFileTest.php of subtitle-toolbox-php.
func TestRollingCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		want     []cueRow
		comments []model.Comment
	}{
		{
			name:    "yt-dlp rolling cues collapse to one cue per line",
			content: fixture(t, filepath.Join(realDir, "own_ytdlp_rolling.vtt")),
			want: []cueRow{
				{160 * ms, 2389 * ms, "welcome<00:00:00.400><c> back</c><00:00:00.800><c> to</c><00:00:01.120><c> the</c><00:00:01.440><c> garden</c>"},
				{2389 * ms, 4870 * ms, "today<00:00:02.880><c> we</c><00:00:03.200><c> plant</c><00:00:03.600><c> tomatoes</c>"},
				{4870 * ms, 6150 * ms, "first"},
				{6150 * ms, 8430 * ms, "dig<00:00:06.640><c> a</c><00:00:06.800><c> small</c><00:00:07.200><c> hole</c>"},
			},
		},
		{
			name: "short repeat cues without word timestamps stay",
			content: "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nthe ferry leaves\n\n" +
				"00:00:02.000 --> 00:00:02.010\nthe ferry leaves\n\n00:00:02.010 --> 00:00:03.000\nthe ferry leaves\nat noon\n",
			want: []cueRow{
				{sec, 2 * sec, "the ferry leaves"},
				{2 * sec, 2010 * ms, "the ferry leaves"},
				{2010 * ms, 3 * sec, "the ferry leaves\nat noon"},
			},
		},
		{
			name: "a comment before a dropped cue goes before the next kept cue",
			content: "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nthe<00:00:01.500><c> ferry</c>\n\n" +
				"00:00:02.000 --> 00:00:02.010\nthe ferry\n\nNOTE second line\n\n" +
				"00:00:02.010 --> 00:00:03.000\nthe ferry\nleaves<00:00:02.500><c> now</c>\n",
			want: []cueRow{
				{sec, 2 * sec, "the<00:00:01.500><c> ferry</c>"},
				{2 * sec, 3 * sec, "leaves<00:00:02.500><c> now</c>"},
			},
			comments: []model.Comment{{Text: "second line", BeforeCue: 1}},
		},
		{
			name: "a repeat cue at the end is dropped",
			content: "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nthe<00:00:01.500><c> ferry</c>\n\n" +
				"00:00:02.000 --> 00:00:02.010\nthe ferry\n\nNOTE end\n",
			want:     []cueRow{{sec, 2 * sec, "the<00:00:01.500><c> ferry</c>"}},
			comments: []model.Comment{{Text: "end", BeforeCue: 1}},
		},
		{
			name: "a repeat cue that lasts 11 ms stays",
			content: "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nthe<00:00:01.500><c> ferry</c>\n\n" +
				"00:00:02.000 --> 00:00:02.011\nthe ferry\n",
			want: []cueRow{{sec, 2 * sec, "the<00:00:01.500><c> ferry</c>"}, {2 * sec, 2011 * ms, "the ferry"}},
		},
		{
			name: "the next cue keeps a first line that differs from the repeat",
			content: "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nthe<00:00:01.500><c> ferry</c>\n\n" +
				"00:00:02.000 --> 00:00:02.010\nthe ferry\n\n00:00:02.500 --> 00:00:03.000\nnew line\nleaves\n",
			want: []cueRow{{sec, 2 * sec, "the<00:00:01.500><c> ferry</c>"}, {2500 * ms, 3 * sec, "new line\nleaves"}},
		},
		{
			name: "plain lines compare without tags, entities and white space runs",
			content: "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n<i>fish</i> &amp;<00:00:01.500><c>  chips</c>\n\n" +
				"00:00:02.000 --> 00:00:02.000\nfish &  chips\n\n00:00:01.500 --> 00:00:03.000\nfish & chips\nnext\n",
			want: []cueRow{{sec, 2 * sec, "<i>fish</i> &amp;<00:00:01.500><c> chips</c>"}, {1500 * ms, 3 * sec, "next"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := parse(t, tt.content, false)
			assert.Equal(t, tt.want, rows(s))
			assert.Equal(t, tt.comments, nilIfNoComments(s.Comments()))
		})
	}
	t.Run("settings of a collapsed cue", func(t *testing.T) {
		t.Parallel()

		s := readFile(t, filepath.Join(realDir, "own_ytdlp_rolling.vtt"))
		c, _ := s.Cue(1)
		assert.Equal(t, vttData(model.Field{Key: "align", Value: "start"}, model.Field{Key: "position", Value: "0%"}), cueData(t, c))
	})
}
