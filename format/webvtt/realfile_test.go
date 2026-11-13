package webvtt_test

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
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

// Ports testRealFileRoundTripKeepsAllData of tests/Parsers/WebVttRealFileTest.php of subtitle-toolbox-php.
func TestRealFilesRoundTripKeepsAllData(t *testing.T) {
	t.Parallel()

	for _, path := range realFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			s := readFile(t, path)
			var out bytes.Buffer
			_, err := webvtt.Write(&out, s, model.CommonWriteOptions{}, webvtt.WriteOptions{})
			require.NoError(t, err)
			again, _ := read(t, out.Bytes(), false)
			assert.Equal(t, describe(t, s), describe(t, again))
			assert.Equal(t, s.Comments(), again.Comments())
			d, _ := s.FormatData("vtt")
			dAgain, _ := again.FormatData("vtt")
			assert.Equal(t, d, dAgain)

			var second bytes.Buffer
			_, err = webvtt.Write(&second, again, model.CommonWriteOptions{}, webvtt.WriteOptions{})
			require.NoError(t, err)
			assert.Equal(t, out.String(), second.String())
		})
	}
}

// describe returns what a round trip keeps of each cue. Cue numbers fill in missing identifiers.
func describe(t *testing.T, s *model.Subtitle) []any {
	t.Helper()

	var out []any
	for _, c := range s.All() {
		out = append(out, []any{c.Start, c.End, c.Lines, c.Position.Alignment, cueData(t, c)})
	}
	return out
}

// Ports the single-file tests of tests/Parsers/WebVttRealFileTest.php of subtitle-toolbox-php. The read goldens hold
// the values that the files read as. These subtests check what a golden cannot, the writer output and a nil slice.
func TestReadRealFileDetails(t *testing.T) {
	t.Parallel()

	t.Run("a cue without text has no lines", func(t *testing.T) {
		t.Parallel()

		s := readFile(t, filepath.Join(realDir, "own_empty_cues.vtt"))
		c, _ := s.Cue(1)
		assert.Nil(t, c.Lines)
	})
	t.Run("hours of any number of digits", func(t *testing.T) {
		t.Parallel()

		s := readFile(t, filepath.Join(realDir, "own_hour_digits.vtt"))
		out := write(t, s, model.CommonWriteOptions{})
		assert.Contains(t, out, "\n10:00:30.940 --> 10:00:40.750\n")
		assert.Contains(t, out, "\n00:00:00.800 --> 00:00:02.933\n")
	})
	t.Run("settings right after the end time", func(t *testing.T) {
		t.Parallel()

		s := readFile(t, filepath.Join(realDir, "own_settings_without_space.vtt"))
		assert.Contains(t, write(t, s, model.CommonWriteOptions{}), "\n00:00:00.000 --> 00:00:01.000 line:40% size:40%\n")
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
}
