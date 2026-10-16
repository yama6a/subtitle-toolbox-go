package sbv_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/sbv"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func cue(start, end time.Duration, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Lines: lines}
}

// Ports testValidFileRoundTrips, testHoursAreWrittenWithoutLeadingZero and testTextWithMarkupCharactersRoundTrips
// of tests/Formatters/SbvFormatterTest.php and tests/Parsers/SbvParserTest.php of subtitle-toolbox-php.
func TestWriteRoundTripsFixtures(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"valid.sbv", "special_characters.sbv", "three_digit_hours.sbv", "from_srt.sbv"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, "sbv/"+file)
			want := string(testfiles.Read(t, "sbv/"+file))
			assert.Equal(t, want, write(t, s, model.CommonWriteOptions{BOM: model.BOMRemove}))
		})
	}

	s := readFile(t, "sbv/two_digit_hours.sbv")
	assert.Equal(t, "0:00:01.500,0:00:04.000\nHello world\n\n1:00:05.000,1:00:07.250\nSecond cue\non two lines\n", write(t, s, model.CommonWriteOptions{}))
}

// Ports testSubRipConvertsToSbvWithoutMarkup of tests/Formatters/SbvFormatterTest.php of subtitle-toolbox-php.
func TestWriteConvertsSubRipWithoutMarkup(t *testing.T) {
	t.Parallel()

	s, warnings, err := srt.Read(bytes.NewReader(testfiles.Read(t, "srt/valid.srt")), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	assert.Equal(t, string(testfiles.Read(t, "sbv/from_srt.sbv")), write(t, s, model.CommonWriteOptions{}))
}

// Ports the loose timing line write of testLenientModeReadsOtherSeparatorsAndFractionsAndWarns
// of tests/Parsers/SbvParserTest.php of subtitle-toolbox-php.
func TestWriteLooseTimingLinesInTheStrictForm(t *testing.T) {
	t.Parallel()

	for _, line := range []string{"0:00:07.980.0:00:11.300", "0:00:07,980,0:00:11,300", "0:00:07.98,0:00:11.3"} {
		t.Run(line, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, line+"\nText\n\n0:00:20.000,0:00:21.000\nMore\n", true)
			assert.Equal(t, "0:00:07.980,0:00:11.300\nText\n\n0:00:20.000,0:00:21.000\nMore\n", write(t, s, model.CommonWriteOptions{}))
		})
	}
}

// Ports testMarkupIsStrippedAndEntitiesAreDecoded, testEscapedCoreTextIsWrittenAsPlainText and
// testLinesThatAreEmptyAfterStrippingAreDropped of tests/Formatters/SbvFormatterTest.php of subtitle-toolbox-php.
func TestWriteText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cues []model.Cue
		want string
	}{
		{
			"markup and entities",
			[]model.Cue{cue(time.Second, 2*time.Second, "<i>Tom</i> &amp; <b>Jerry</b>", "&lt;3 caf&eacute; &quot;ok&quot; &#39;yes&#39;")},
			"0:00:01.000,0:00:02.000\nTom & Jerry\n<3 café \"ok\" 'yes'\n",
		},
		{
			"escaped core text",
			[]model.Cue{cue(time.Second, 2*time.Second, "I &lt;3 bread &amp; jam")},
			"0:00:01.000,0:00:02.000\nI <3 bread & jam\n",
		},
		{
			"empty lines drop and cues without text are skipped",
			[]model.Cue{
				cue(time.Second, 2*time.Second, "<i></i>", "Text"),
				cue(3*time.Second, 4*time.Second, "<b> </b>"),
				cue(4*time.Second, 5*time.Second),
				cue(5*time.Second, 6*time.Second, "Last"),
			},
			"0:00:01.000,0:00:02.000\nText\n\n0:00:05.000,0:00:06.000\nLast\n",
		},
		{
			"speakers, word timestamps and ruby",
			[]model.Cue{cue(0, time.Second, "<v Anna>Hello <00:00:00.500>there</v>", "<ruby>base<rt>note</rt></ruby>")},
			"0:00:00.000,0:00:01.000\nHello there\nbase (note)\n",
		},
		{
			"lines are trimmed",
			[]model.Cue{cue(0, time.Second, "  a  ", "<i> b </i>")},
			"0:00:00.000,0:00:01.000\na\nb\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, subtitle(t, tt.cues...), model.CommonWriteOptions{}))
		})
	}
}

func TestWriteTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		start, end time.Duration
		want       string
	}{
		{"zero", 0, 0, "0:00:00.000,0:00:00.000"},
		{"one hour digit up to 9 h", 9*time.Hour + 59*time.Minute + 59999*ms, 9*time.Hour + 59*time.Minute + 59999*ms, "9:59:59.999,9:59:59.999"},
		{"two hour digits from 10 h", 10 * time.Hour, 99 * time.Hour, "10:00:00.000,99:00:00.000"},
		{"three hour digits", 100*time.Hour + 1500*ms, 99999 * time.Hour, "100:00:01.500,99999:00:00.000"},
		{"negative time", -time.Second, 2 * time.Second, "0:00:00.000,0:00:02.000"},
		{"microseconds round", 1499999 * time.Nanosecond, 1500500 * time.Nanosecond, "0:00:00.001,0:00:00.002"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want+"\nText\n", write(t, subtitle(t, cue(tt.start, tt.end, "Text")), model.CommonWriteOptions{}))
		})
	}
}

func TestWriteOutputOptions(t *testing.T) {
	t.Parallel()

	s := subtitle(t, cue(time.Second, 2*time.Second, "<i>A</i>"), cue(3*time.Second, 4*time.Second, "B"))
	tests := []struct {
		name   string
		common model.CommonWriteOptions
		want   string
	}{
		{"default has no BOM", model.CommonWriteOptions{}, "0:00:01.000,0:00:02.000\nA\n\n0:00:03.000,0:00:04.000\nB\n"},
		{"BOM added", model.CommonWriteOptions{BOM: model.BOMAdd}, "\xEF\xBB\xBF0:00:01.000,0:00:02.000\nA\n\n0:00:03.000,0:00:04.000\nB\n"},
		{"CR LF", model.CommonWriteOptions{LineEnding: model.CRLF}, "0:00:01.000,0:00:02.000\r\nA\r\n\r\n0:00:03.000,0:00:04.000\r\nB\r\n"},
		{"strip tags changes nothing", model.CommonWriteOptions{StripTags: true}, "0:00:01.000,0:00:02.000\nA\n\n0:00:03.000,0:00:04.000\nB\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, s, tt.common))
		})
	}
}

func TestWriteNoCues(t *testing.T) {
	t.Parallel()

	assert.Empty(t, write(t, &model.Subtitle{}, model.CommonWriteOptions{}))
	assert.Empty(t, write(t, subtitle(t, cue(0, time.Second)), model.CommonWriteOptions{}))
}

func TestWriteFailsOnWriterError(t *testing.T) {
	t.Parallel()

	err := sbv.Write(errWriter{}, subtitle(t, cue(0, time.Second, "A")), model.CommonWriteOptions{}, sbv.WriteOptions{})
	require.ErrorIs(t, err, errRead)
}

// The ruby fixture is the SBV output of the shared ruby input of tests/Formatters/SbvFormatterTest.php of subtitle-toolbox-php.
func TestWriteRubyFixture(t *testing.T) {
	t.Parallel()

	s, warnings, err := webvtt.Read(strings.NewReader(string(testfiles.Read(t, "ruby/own_ruby.vtt"))), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	assert.Equal(t, string(testfiles.Read(t, "ruby/own_ruby_expected.sbv")), write(t, s, model.CommonWriteOptions{}))
}
