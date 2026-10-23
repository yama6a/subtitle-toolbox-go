package plaintext_test

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/plaintext"
	"github.com/yama6a/subtitle-toolbox-go/format/sbv"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var update = flag.Bool("update", false, "rewrite the write golden files")

const ms = time.Millisecond

func subtitle(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func cue(start, end time.Duration, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Lines: lines}
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts plaintext.WriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, plaintext.Write(&out, s, common, opts))
	return out.String()
}

func walk() *model.Subtitle {
	return subtitle(
		cue(time.Second, 2*time.Second, "<v Mara><b>Hello</b>", "world."),
		cue(2500*ms, 4*time.Second, "Where are you  going?"),
		cue(6*time.Second, 7*time.Second, `<font color="#ff0000">Home</font> &amp; <00:00:06.500>bed.`, "  "),
		cue(3725*time.Second, 3727*time.Second, "Fish &lt;3"),
	)
}

// Ports testFormatsWithOptions of tests/Formatters/PlainTextFormatterTest.php of subtitle-toolbox-php.
// PHP passes paragraphGap INF where the Go option is NoParagraphBreaks.
func TestWriteOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		common model.CommonWriteOptions
		opts   plaintext.WriteOptions
		want   string
	}{
		{name: "defaults", want: "Hello world. Where are you going?\n\nHome & bed.\n\nFish <3\n"},
		{name: "lines kept", opts: plaintext.WriteOptions{KeepLineBreaks: true}, want: "Hello\nworld. Where are you going?\n\nHome & bed.\n\nFish <3\n"},
		{name: "one cue per line", opts: plaintext.WriteOptions{OneCuePerLine: true}, want: "Hello world.\nWhere are you going?\n\nHome & bed.\n\nFish <3\n"},
		{name: "gap of 2.5 s", opts: plaintext.WriteOptions{ParagraphGap: new(2500 * ms)}, want: "Hello world. Where are you going? Home & bed.\n\nFish <3\n"},
		{name: "gap of 0.5 s", opts: plaintext.WriteOptions{ParagraphGap: new(500 * ms)}, want: "Hello world.\n\nWhere are you going?\n\nHome & bed.\n\nFish <3\n"},
		{name: "no paragraphs", opts: plaintext.WriteOptions{ParagraphGap: new(plaintext.NoParagraphBreaks)}, want: "Hello world. Where are you going? Home & bed. Fish <3\n"},
		{
			name: "with times", opts: plaintext.WriteOptions{WithTimes: true},
			want: "[00:00:01] Hello world. Where are you going?\n\n[00:00:06] Home & bed.\n\n[01:02:05] Fish <3\n",
		},
		{
			name: "CR LF", common: model.CommonWriteOptions{LineEnding: model.CRLF}, opts: plaintext.WriteOptions{ParagraphGap: new(plaintext.NoParagraphBreaks)},
			want: "Hello world. Where are you going? Home & bed. Fish <3\r\n",
		},
		{
			name: "CR LF between paragraphs and kept lines", common: model.CommonWriteOptions{LineEnding: model.CRLF}, opts: plaintext.WriteOptions{KeepLineBreaks: true, OneCuePerLine: true},
			want: "Hello\r\nworld.\r\nWhere are you going?\r\n\r\nHome & bed.\r\n\r\nFish <3\r\n",
		},
		{name: "BOM add", common: model.CommonWriteOptions{BOM: model.BOMAdd}, opts: plaintext.WriteOptions{ParagraphGap: new(plaintext.NoParagraphBreaks)}, want: "\ufeffHello world. Where are you going? Home & bed. Fish <3\n"},
		{name: "gap 0 splits at every cue that starts at or after the latest end", opts: plaintext.WriteOptions{ParagraphGap: new(time.Duration(0))}, want: "Hello world.\n\nWhere are you going?\n\nHome & bed.\n\nFish <3\n"},
		{
			name: "all options", opts: plaintext.WriteOptions{KeepLineBreaks: true, OneCuePerLine: true, WithTimes: true, ParagraphGap: new(2500 * ms)},
			want: "[00:00:01] Hello\nworld.\nWhere are you going?\nHome & bed.\n\n[01:02:05] Fish <3\n",
		},
		{name: "StripTags changes nothing", common: model.CommonWriteOptions{StripTags: true}, want: "Hello world. Where are you going?\n\nHome & bed.\n\nFish <3\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, walk(), tt.common, tt.opts))
		})
	}
}

// Ports testMeasuresTheGapFromTheLatestEnd of tests/Formatters/PlainTextFormatterTest.php of subtitle-toolbox-php.
func TestGapCountsFromTheLatestEnd(t *testing.T) {
	t.Parallel()

	s := subtitle(cue(0, 10*time.Second, "Long"), cue(time.Second, 2*time.Second, "Short"), cue(11*time.Second, 12*time.Second, "Overlapped"))
	assert.Equal(t, "Long Short Overlapped\n", write(t, s, model.CommonWriteOptions{}, plaintext.WriteOptions{}))
}

// Ports testSkipsCuesWithoutTextAndWritesNothingForNoCues of tests/Formatters/PlainTextFormatterTest.php of
// subtitle-toolbox-php. The image cue of PHP has no counterpart in Go.
func TestCuesWithoutText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    *model.Subtitle
		opts plaintext.WriteOptions
		want string
	}{
		{name: "no cues", s: &model.Subtitle{}},
		{name: "empty tag pair and no lines", s: subtitle(cue(time.Second, 2*time.Second, "<i></i>"), cue(3*time.Second, 4*time.Second)), want: ""},
		{name: "white space only", s: subtitle(cue(time.Second, 2*time.Second, " \t", "&#32;", "<b> </b>")), want: ""},
		{
			name: "a skipped cue does not take part in the paragraphs",
			s:    subtitle(cue(0, time.Second, "A"), cue(time.Second, 20*time.Second), cue(3*time.Second, 4*time.Second, "B")),
			want: "A\n\nB\n",
		},
		{
			name: "a skipped cue does not start a paragraph with times",
			s:    subtitle(cue(0, time.Second, "<i></i>"), cue(5*time.Second, 6*time.Second, "B")),
			opts: plaintext.WriteOptions{WithTimes: true},
			want: "[00:00:05] B\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, tt.s, model.CommonWriteOptions{}, tt.opts))
		})
	}
}

func TestLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		opts  plaintext.WriteOptions
		want  string
	}{
		{name: "tags and word timestamps", lines: []string{"<i>a</i> <00:00:01.000>b <c.yellow>c</c>"}, want: "a b c\n"},
		{name: "entities are decoded after the tags go", lines: []string{"&lt;i&gt;x&lt;/i&gt; &amp;amp; &#233;"}, want: "<i>x</i> &amp; \u00e9\n"},
		{name: "runs of spaces and tabs", lines: []string{"a \t  b\t\tc"}, want: "a b c\n"},
		{name: "a decoded space run collapses", lines: []string{"a&#32;&#32;b&#9;c"}, want: "a b c\n"},
		{name: "no-break space stays", lines: []string{"a\u00a0\u00a0b"}, want: "a\u00a0\u00a0b\n"},
		{name: "trim set", lines: []string{"\x00\x0b a \x00\x0b\r"}, want: "a\n"},
		{name: "empty lines drop out", lines: []string{"", "a", "  ", "<b></b>", "b"}, opts: plaintext.WriteOptions{KeepLineBreaks: true}, want: "a\nb\n"},
		{name: "ruby as text", lines: []string{"<ruby>base<rt>note</rt></ruby> text"}, want: "base (note) text\n"},
		{name: "voice tag", lines: []string{"<v Mara>Hi"}, want: "Hi\n"},
		{name: "a line feed inside a line stays", lines: []string{"a&#10;b"}, want: "a\nb\n"},
		{name: "invalid UTF-8 stays", lines: []string{"a\xffb"}, want: "a\xffb\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, subtitle(cue(0, time.Second, tt.lines...)), model.CommonWriteOptions{}, tt.opts))
		})
	}
}

func TestParagraphStartTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		start time.Duration
		want  string
	}{
		{name: "floored to whole seconds", start: 1999 * ms, want: "[00:00:01] A\n"},
		{name: "below 1 s", start: 999 * ms, want: "[00:00:00] A\n"},
		{name: "negative", start: -1500 * ms, want: "[00:00:00] A\n"},
		{name: "minutes", start: 59*time.Minute + 59*time.Second, want: "[00:59:59] A\n"},
		{name: "99 hours", start: 99 * time.Hour, want: "[99:00:00] A\n"},
		{name: "100 hours", start: 100*time.Hour + time.Second, want: "[100:00:01] A\n"},
		{name: "largest time", start: 100000*time.Hour - ms, want: "[99999:59:59] A\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(cue(tt.start, tt.start+time.Second, "A"))
			assert.Equal(t, tt.want, write(t, s, model.CommonWriteOptions{}, plaintext.WriteOptions{WithTimes: true}))
		})
	}
}

func TestParagraphGap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		gap  *time.Duration
		cues []model.Cue
		want string
	}{
		{name: "nil is 2 s, just below", cues: []model.Cue{cue(0, time.Second, "A"), cue(2999*ms, 4*time.Second, "B")}, want: "A B\n"},
		{name: "nil is 2 s, equal", cues: []model.Cue{cue(0, time.Second, "A"), cue(3*time.Second, 4*time.Second, "B")}, want: "A\n\nB\n"},
		{name: "a decimal gap that PHP floats compare below", gap: new(2 * time.Second), cues: []model.Cue{cue(time.Second, 1300*ms, "A"), cue(3300*ms, 4*time.Second, "B")}, want: "A\n\nB\n"},
		{name: "0.1 s", gap: new(100 * ms), cues: []model.Cue{cue(100*ms, 200*ms, "A"), cue(300*ms, 400*ms, "B")}, want: "A\n\nB\n"},
		{name: "overlap at gap 0 joins", gap: new(time.Duration(0)), cues: []model.Cue{cue(0, 2*time.Second, "A"), cue(time.Second, 3*time.Second, "B")}, want: "A B\n"},
		{name: "a cue that ends before it starts leaves the latest end at its end", gap: new(time.Second), cues: []model.Cue{cue(5*time.Second, 0, "A"), cue(5500*ms, 6*time.Second, "B")}, want: "A\n\nB\n"},
		{name: "far apart with no breaks", gap: new(plaintext.NoParagraphBreaks), cues: []model.Cue{cue(0, time.Second, "A"), cue(99999*time.Hour, 99999*time.Hour+time.Second, "B")}, want: "A B\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, subtitle(tt.cues...), model.CommonWriteOptions{}, plaintext.WriteOptions{ParagraphGap: tt.gap}))
		})
	}
}

// Ports testThrowsForANegativeParagraphGap of tests/Formatters/PlainTextFormatterTest.php of subtitle-toolbox-php.
func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		gap  *time.Duration
		want error
	}{
		{name: "nil", gap: nil},
		{name: "zero", gap: new(time.Duration(0))},
		{name: "2 s", gap: new(2 * time.Second)},
		{name: "no paragraph breaks", gap: new(plaintext.NoParagraphBreaks)},
		{
			name: "minus 1 s", gap: new(-time.Second),
			want: &model.InvalidArgumentError{Option: "WriteOptions.PlainText.ParagraphGap", Msg: "The paragraph gap must be 0 or more seconds, got -1."},
		},
		{
			name: "minus 1 ms", gap: new(-ms),
			want: &model.InvalidArgumentError{Option: "WriteOptions.PlainText.ParagraphGap", Msg: "The paragraph gap must be 0 or more seconds, got -0.001."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := plaintext.WriteOptions{ParagraphGap: tt.gap}
			err := opts.Validate()
			if tt.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			assert.Equal(t, tt.want, err)

			var out bytes.Buffer
			require.ErrorIs(t, plaintext.Write(&out, walk(), model.CommonWriteOptions{}, opts), model.ErrInvalidArgument)
			assert.Empty(t, out.String())
		})
	}
}

var errDisk = errors.New("disk gone")

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errDisk }

func TestWriteFailsOnWriterError(t *testing.T) {
	t.Parallel()

	err := plaintext.Write(errWriter{}, walk(), model.CommonWriteOptions{}, plaintext.WriteOptions{})
	require.ErrorIs(t, err, errDisk)
	assert.Contains(t, err.Error(), "txt")
}

// The two real files are the PHP output for sbv/real/youtube_studio_lf.sbv. Ports testRealFileGivesTheExpectedTranscript
// of tests/Formatters/PlainTextFormatterTest.php of subtitle-toolbox-php. PHP calls the options
// withTimes: true, joinCues: false.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file string
		opts plaintext.WriteOptions
	}{
		{"youtube_studio_lf.txt", plaintext.WriteOptions{}},
		{"youtube_studio_lf_with_times.txt", plaintext.WriteOptions{WithTimes: true, OneCuePerLine: true}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := sbv.Read(bytes.NewReader(testfiles.Read(t, "sbv/real/youtube_studio_lf.sbv")), model.CommonReadOptions{}, sbv.ReadOptions{})
			require.NoError(t, err)
			require.Empty(t, warnings)
			var out bytes.Buffer
			require.NoError(t, plaintext.Write(&out, s, model.CommonWriteOptions{}, tt.opts))
			golden.Equal(t, testfiles.Path(t, "plaintext/real/"+tt.file), out.Bytes(), *update)
		})
	}
}

// The ruby fixture is the plain text output of the shared ruby input of tests/Formatters/PlainTextFormatterTest.php.
func TestRubyFixture(t *testing.T) {
	t.Parallel()

	s, warnings, err := webvtt.Read(strings.NewReader(string(testfiles.Read(t, "ruby/own_ruby.vtt"))), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	assert.Equal(t, string(testfiles.Read(t, "ruby/own_ruby_expected.txt")), write(t, s, model.CommonWriteOptions{}, plaintext.WriteOptions{}))
}
