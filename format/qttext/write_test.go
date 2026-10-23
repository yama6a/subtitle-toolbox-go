package qttext_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/qttext"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func cue(start, end time.Duration, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Lines: lines}
}

// Adapted from testConvertingToFormat of mantas-done/subtitles, tests/formats/TxtQuickTimeTest.php
// (https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/formats/TxtQuickTimeTest.php), MIT.
// Ports testWritesTheSubtitleEditShapeForANewFile of tests/Formatters/QuickTimeTextFormatterTest.php of
// subtitle-toolbox-php.
func TestWriteSubtitleEditShapeForANewFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    *model.Subtitle
		want string
	}{
		{
			"mantas-done Subtitle Edit shape",
			subtitle(
				cue(137440*ms, 140375*ms, "The ferry leaves at nine,", "from the north pier."),
				cue(3740476*ms, 3742501*ms, "Tickets at the kiosk."),
			),
			subtitleEditHeader +
				"[00:02:17.13]\nThe ferry leaves at nine,\nfrom the north pier.\n[00:02:20.11]\n\n" +
				"[01:02:20.14]\nTickets at the kiosk.\n[01:02:22.15]\n\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, tt.s, model.CommonWriteOptions{}))
		})
	}
}

// Ports testWritesStylesAsDescriptors of tests/Formatters/QuickTimeTextFormatterTest.php of subtitle-toolbox-php.
func TestWriteStylesAsDescriptors(t *testing.T) {
	t.Parallel()

	s := subtitle(
		cue(time.Second, 2*time.Second, "<i>Sung</i> and <b>loud</b>", `<font color="red">red</font> &amp; plain`),
		cue(3*time.Second, 4*time.Second, "<i>Sung</i>"),
		cue(5*time.Second, 6*time.Second, "<i>Sung <u>loud</u></i>"),
	)
	assert.Equal(t, subtitleEditHeader+
		"[00:00:01.00]\n{italic}Sung{plain} and {bold}loud\n{plain}red & plain\n[00:00:02.00]\n\n"+
		"[00:00:03.00]\n{italic}Sung\n[00:00:04.00]\n\n[00:00:05.00]\nSung {underline}loud\n[00:00:06.00]\n\n", write(t, s, model.CommonWriteOptions{}))
}

// Ports testWritesTheTimeScaleAndTheRelativeTimestampsOfTheHeader of tests/Formatters/QuickTimeTextFormatterTest.php
// of subtitle-toolbox-php.
func TestWriteRelativeTimestamps(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "{QTtext}{timeScale:600}{timeStamps:relative}\n[00:00:01.300]\nOne\n[00:00:02.000]\n", false)
	s.AddCues(cue(4500*ms, 6500*ms, "Two"))
	assert.Equal(t,
		"{QTtext}{timeScale:600}{timeStamps:relative}\n[00:00:00.000]\n[00:00:01.300]\nOne\n[00:00:02.000]\n[00:00:01.000]\nTwo\n[00:00:02.000]\n",
		write(t, s, model.CommonWriteOptions{}))
}

// Ports testWritesMillisecondsWithoutATimeScale of tests/Formatters/QuickTimeTextFormatterTest.php of
// subtitle-toolbox-php.
func TestWriteMillisecondsWithoutATimeScale(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "{QTtext}{font:Geneva}\n[00:00:07.25]\nRain\n[00:00:09.5]\n", false)
	assert.Equal(t, "{QTtext}{font:Geneva}\n[00:00:00.000]\n[00:00:07.250]\nRain\n[00:00:09.500]\n", write(t, s, model.CommonWriteOptions{}))
}

// Ports testWritesTheTextOfAChangedCueFromItsMarkup of tests/Formatters/QuickTimeTextFormatterTest.php of
// subtitle-toolbox-php.
func TestWriteChangedCueUsesItsMarkupNotItsSource(t *testing.T) {
	t.Parallel()

	content := "{QTtext}{timeScale:30}\n[00:00:01.00]\n{font:Geneva}{italic}Sung\n[00:00:02.00]\n"
	s, _ := mustRead(t, content, false)
	assert.Equal(t, "{QTtext}{timeScale:30}\n[00:00:00.00]\n[00:00:01.00]\n{font:Geneva}{italic}Sung\n[00:00:02.00]\n", write(t, s, model.CommonWriteOptions{}))

	first, _ := s.Cue(0)
	first.Lines = []string{"Spoken"}
	require.NoError(t, s.SetCue(0, first))
	assert.Equal(t, "{QTtext}{timeScale:30}\n[00:00:00.00]\n[00:00:01.00]\nSpoken\n[00:00:02.00]\n", write(t, s, model.CommonWriteOptions{}))
}

func TestWriteKeepsTheSourceWhenOnlyTheMarkupDiffers(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "{QTtext}{timeScale:30}\n[00:00:01.00]\n{font:Geneva}{italic}Sung\n[00:00:02.00]\n", false)
	first, _ := s.Cue(0)
	first.Lines = []string{"<i>Sung</i>"}
	require.NoError(t, s.SetCue(0, first))
	assert.Contains(t, write(t, s, model.CommonWriteOptions{}), "\n{font:Geneva}{italic}Sung\n")
}

func TestWriteKeepsTheSourceOfAnUnchangedCue(t *testing.T) {
	t.Parallel()

	content := "{QTtext}{timeScale:30}\n[00:00:00.00]\n[00:00:01.00]\n{font:Geneva}{italic}Sung\n  {x:y}more\n[00:00:02.00]\n[00:00:03.00]\nStill sung\n[00:00:04.00]\n"
	s, _ := mustRead(t, content, false)
	assert.Equal(t, content, write(t, s, model.CommonWriteOptions{}))
}

func TestWriteSourceTakesTheStyleOfTheSource(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "{QTtext}{timeScale:30}\n[00:00:00.00]\n[00:00:01.00]\n{italic}Sung\n[00:00:02.00]\n", false)
	s.AddCues(cue(3*time.Second, 4*time.Second, "<i>Still sung</i>"), cue(5*time.Second, 6*time.Second, "Spoken"))
	assert.Equal(t,
		"{QTtext}{timeScale:30}\n[00:00:00.00]\n[00:00:01.00]\n{italic}Sung\n[00:00:02.00]\n[00:00:03.00]\nStill sung\n[00:00:04.00]\n[00:00:05.00]\n{plain}Spoken\n[00:00:06.00]\n",
		write(t, s, model.CommonWriteOptions{}))
}

func TestWriteHeaderStyleIsTheStartingStyle(t *testing.T) {
	t.Parallel()

	s := withHeader(t, []string{"{QTtext}{italic}"}, false, cue(0, time.Second, "<i>Sung</i>"), cue(2*time.Second, 3*time.Second, "<i>Sung</i> and <b>loud</b>"))
	assert.Equal(t, "{QTtext}{italic}\n[00:00:00.000]\nSung\n[00:00:01.000]\n[00:00:02.000]\nSung{plain} and {bold}loud\n[00:00:03.000]\n",
		write(t, s, model.CommonWriteOptions{}))
}

func TestWriteLayouts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		s      *model.Subtitle
		layout string
	}{
		{
			"QuickTime layout starts with an empty sample",
			withHeader(t, []string{"{QTtext}{timeScale:30}"}, false, cue(time.Second, 2*time.Second, "A")),
			"[00:00:00.00]\n[00:00:01.00]\nA\n[00:00:02.00]\n",
		},
		{
			"QuickTime layout writes the first start at 0",
			withHeader(t, []string{"{QTtext}{timeScale:30}"}, false, cue(0, 2*time.Second, "A")),
			"[00:00:00.00]\nA\n[00:00:02.00]\n",
		},
		{
			"QuickTime layout writes no start at the end of the cue before",
			withHeader(t, []string{"{QTtext}{timeScale:30}"}, false, cue(time.Second, 2*time.Second, "A"), cue(2*time.Second, 3*time.Second, "B")),
			"[00:00:00.00]\n[00:00:01.00]\nA\n[00:00:02.00]\nB\n[00:00:03.00]\n",
		},
		{
			"QuickTime layout compares the start in units",
			withHeader(t, []string{"{QTtext}{timeScale:30}"}, false, cue(time.Second, 2*time.Second, "A"), cue(2*time.Second+10*ms, 3*time.Second, "B")),
			"[00:00:00.00]\n[00:00:01.00]\nA\n[00:00:02.00]\nB\n[00:00:03.00]\n",
		},
		{
			"QuickTime layout writes a start after a gap",
			withHeader(t, []string{"{QTtext}{timeScale:30}"}, false, cue(time.Second, 2*time.Second, "A"), cue(3*time.Second, 4*time.Second, "B")),
			"[00:00:00.00]\n[00:00:01.00]\nA\n[00:00:02.00]\n[00:00:03.00]\nB\n[00:00:04.00]\n",
		},
		{
			"QuickTime layout writes a start before the end of the cue before",
			withHeader(t, []string{"{QTtext}{timeScale:30}"}, false, cue(time.Second, 3*time.Second, "A"), cue(2*time.Second, 4*time.Second, "B")),
			"[00:00:00.00]\n[00:00:01.00]\nA\n[00:00:03.00]\n[00:00:02.00]\nB\n[00:00:04.00]\n",
		},
		{
			"paired layout from the stored flag",
			withHeader(t, []string{"{QTtext}{timeScale:30}"}, true, cue(time.Second, 2*time.Second, "A"), cue(2*time.Second, 3*time.Second, "B")),
			"[00:00:01.00]\nA\n[00:00:02.00]\n\n[00:00:02.00]\nB\n[00:00:03.00]\n\n",
		},
		{"no cues", withHeader(t, []string{"{QTtext}{timeScale:30}"}, false), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, "{QTtext}{timeScale:30}\n"+tt.layout, write(t, tt.s, model.CommonWriteOptions{}))
		})
	}
}

func TestWriteEmptySubtitleWritesTheDefaultHeader(t *testing.T) {
	t.Parallel()

	assert.Equal(t, subtitleEditHeader, write(t, &model.Subtitle{}, model.CommonWriteOptions{}))
}

func TestWriteTimestampDigits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		header     string
		start, end string
	}{
		{"{QTtext}{timeScale:1}", "[00:00:00.0]", "[00:00:02.0]"},
		{"{QTtext}{timeScale:2}", "[00:00:00.0]", "[00:00:01.1]"},
		{"{QTtext}{timeScale:10}", "[00:00:00.0]", "[00:00:01.5]"},
		{"{QTtext}{timeScale:11}", "[00:00:00.00]", "[00:00:01.06]"},
		{"{QTtext}{timeScale:30}", "[00:00:00.00]", "[00:00:01.15]"},
		{"{QTtext}{timeScale:100}", "[00:00:00.00]", "[00:00:01.50]"},
		{"{QTtext}{timeScale:101}", "[00:00:00.000]", "[00:00:01.051]"},
		{"{QTtext}{timeScale:600}", "[00:00:00.000]", "[00:00:01.300]"},
		{"{QTtext}{timeScale:1000}", "[00:00:00.000]", "[00:00:01.500]"},
		{"{QTtext}{timeScale:48000}", "[00:00:00.00000]", "[00:00:01.24000]"},
		{"{QTtext}", "[00:00:00.000]", "[00:00:01.500]"},
		{"{QTtext}{timeScale:0}", "[00:00:00.000]", "[00:00:01.500]"},
		{"{QTtext}{timeScale:fast}", "[00:00:00.000]", "[00:00:01.500]"},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			t.Parallel()

			s := withHeader(t, []string{tt.header}, false, cue(0, 1500*ms, "A"))
			assert.Equal(t, tt.header+"\n"+tt.start+"\nA\n"+tt.end+"\n", write(t, s, model.CommonWriteOptions{}))
		})
	}
}

func TestWriteRoundsHalfAwayFromZero(t *testing.T) {
	t.Parallel()

	s := withHeader(t, []string{"{QTtext}{timeScale:30}"}, true, cue(0, 17*ms, "A"), cue(0, 16*ms, "B"), cue(50*ms, 1050*ms, "C"))
	assert.Equal(t,
		"{QTtext}{timeScale:30}\n[00:00:00.00]\nA\n[00:00:00.01]\n\n[00:00:00.00]\nB\n[00:00:00.00]\n\n[00:00:00.02]\nC\n[00:00:01.02]\n\n",
		write(t, s, model.CommonWriteOptions{}))
}

// Up to 1000 units per second the writer rounds as PHP 8.5 does, in floats, so 64.82 s at 25 per second is 1620.5
// units in decimal and 1620.4999999999998 in binary.
func TestWriteRoundsAsPHPDoesUpTo1000UnitsPerSecond(t *testing.T) {
	t.Parallel()

	s := withHeader(t, []string{"{QTtext}{timeScale:25}"}, true, cue(64820*ms, 64820*ms, "A"))
	assert.Equal(t, "{QTtext}{timeScale:25}\n[00:01:04.20]\nA\n[00:01:04.20]\n\n", write(t, s, model.CommonWriteOptions{}))
}

func TestWriteRoundsHalfAwayFromZeroAbove1000UnitsPerSecond(t *testing.T) {
	t.Parallel()

	s := withHeader(t, []string{"{QTtext}{timeScale:2500}"}, true, cue(ms, 3*ms, "A"))
	assert.Equal(t, "{QTtext}{timeScale:2500}\n[00:00:00.0003]\nA\n[00:00:00.0008]\n\n", write(t, s, model.CommonWriteOptions{}))
}

func TestWriteClampsTimes(t *testing.T) {
	t.Parallel()

	s := withHeader(t, []string{"{QTtext}{timeScale:30}"}, true, cue(-time.Second, -2*time.Second, "A"), cue(time.Second, 500*ms, "B"))
	assert.Equal(t,
		"{QTtext}{timeScale:30}\n[00:00:00.00]\nA\n[00:00:00.00]\n\n[00:00:01.00]\nB\n[00:00:01.00]\n\n",
		write(t, s, model.CommonWriteOptions{}))
}

func TestWriteRelativeTimestampsCannotGoBack(t *testing.T) {
	t.Parallel()

	s := withHeader(t, []string{"{QTtext}{timeStamps:relative}"}, false, cue(time.Second, 3*time.Second, "A"), cue(2*time.Second, 4*time.Second, "B"))
	assert.Equal(t,
		"{QTtext}{timeStamps:relative}\n[00:00:00.000]\n[00:00:01.000]\nA\n[00:00:02.000]\n[00:00:00.000]\nB\n[00:00:01.000]\n",
		write(t, s, model.CommonWriteOptions{}))
}

func TestWriteText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"plain lines", []string{"One", "Two"}, "One\nTwo\n"},
		{"tags other than b, i and u go", []string{`<font color="red">Red</font> <s>struck</s> <v Anna>x</v>`}, "Red struck x\n"},
		{"the tags in any case", []string{"<I>a</I> <B>b</B> <U>c</U>"}, "{italic}a {plain}{bold}b {plain}{underline}c\n"},
		{"all 3 styles open", []string{"<b><i><u>All</u></i></b>"}, "{bold}{italic}{underline}All\n"},
		{"a style that changes inside a line", []string{"<b>a</b> b <i>c</i>"}, "{bold}a{plain} b {italic}c\n"},
		{"a style that turns off keeps the others", []string{"<b><i>a</i></b> <b>b</b>"}, "{bold}{italic}a {plain}{bold}b\n"},
		{"a style across lines", []string{"<b>one", "two</b>"}, "{bold}one\ntwo\n"},
		{"a style that only wraps white space writes no descriptor", []string{"a <b> </b> b"}, "a   b\n"},
		{"entities decode", []string{"a &amp; b &lt;i&gt; &#233;"}, "a & b <i> \u00e9\n"},
		{"each line is trimmed", []string{"  One  ", " \t "}, "One\n"},
		{"ruby becomes text", []string{"<ruby>kanji<rt>kan</rt></ruby>"}, "kanji (kan)\n"},
		{"a cue without visible text writes only the stamps", []string{"<b></b>", ""}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := withHeader(t, []string{"{QTtext}"}, false, cue(0, time.Second, tt.lines...))
			assert.Equal(t, "{QTtext}\n[00:00:00.000]\n"+tt.want+"[00:00:01.000]\n", write(t, s, model.CommonWriteOptions{}))
		})
	}
}

func TestWriteOutputOptions(t *testing.T) {
	t.Parallel()

	s := subtitle(cue(0, time.Second, "A"))
	plain := write(t, s, model.CommonWriteOptions{})
	assert.False(t, strings.HasPrefix(plain, "\xEF\xBB\xBF"))
	assert.Equal(t, "\xEF\xBB\xBF"+plain, write(t, s, model.CommonWriteOptions{BOM: model.BOMAdd}))
	assert.Equal(t, strings.ReplaceAll(plain, "\n", "\r\n"), write(t, s, model.CommonWriteOptions{LineEnding: model.CRLF}))
}

func TestWriteHeaderAlwaysReadsBack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header []string
		want   string
	}{
		{"CR and LF become spaces", []string{"{QTtext}", "a\nb\rc\r\nd"}, "{QTtext}\na b c  d\n"},
		{"a header line that reads as a timestamp is not written", []string{"{QTtext}", "[00:00:01.00]", " [1:02:03] x", "{plain}"}, "{QTtext}\n{plain}\n"},
		{"an LF that makes a timestamp line", []string{"{QTtext}", "\n[00:00:09.00]Injected"}, "{QTtext}\n"},
		{"no header lines", nil, "{QTtext}\n"},
		{"the first line is no {QTtext}", []string{"{timeScale:30}"}, "{QTtext}\n{timeScale:30}\n"},
		{"a timestamp as the first line", []string{"[00:00:01.00]", "{timeScale:30}"}, "{QTtext}\n{timeScale:30}\n"},
		{"{QTtext} in lower case after white space", []string{" \t{qttext}"}, " \t{qttext}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := withHeader(t, tt.header, false)
			got := write(t, s, model.CommonWriteOptions{})
			assert.Equal(t, tt.want, got)
			back, warnings, err := read(got, model.CommonReadOptions{})
			require.NoError(t, err)
			assert.Empty(t, warnings)
			assert.Zero(t, back.Len())
		})
	}
}

func TestWriteStaysBelowTheTimeLimit(t *testing.T) {
	t.Parallel()

	for _, scale := range []string{"", "1", "2", "10", "30", "600", "1000", "1001", "48000", "1000000", "99999999999999999999"} {
		t.Run("scale "+scale, func(t *testing.T) {
			t.Parallel()

			header := "{QTtext}"
			if scale != "" {
				header += "{timeScale:" + scale + "}"
			}
			for _, limit := range []time.Duration{timecode.MaxValidTime, timecode.MaxValidTime - ms, timecode.MaxValidTime - 17*ms, timecode.MaxTime - time.Second} {
				s := withHeader(t, []string{header}, true, cue(limit-2*time.Second, limit, "A"), cue(limit, limit+time.Hour, "B"))
				back, warnings, err := read(write(t, s, model.CommonWriteOptions{}), model.CommonReadOptions{})
				require.NoError(t, err, "limit %v", limit)
				assert.Empty(t, warnings)
				require.Equal(t, 2, back.Len())
				for i, c := range back.All() {
					assert.Less(t, c.End, timecode.MaxTime)
					want, _ := s.Cue(i)
					unit := ms
					if n, err := strconv.ParseInt(scale, 10, 64); err == nil && n > 0 && n < 1000 {
						unit = time.Second / time.Duration(n)
					}
					want.End = min(want.End, timecode.MaxValidTime)
					assert.InDelta(t, float64(want.End), float64(c.End), float64(unit+ms), "scale %q limit %v", scale, limit)
				}
			}
		})
	}
}

func TestWriteRelativeTimestampsStayBelowTheTimeLimit(t *testing.T) {
	t.Parallel()

	for _, scale := range []string{"1", "30", "600"} {
		header := "{QTtext}{timeScale:" + scale + "}{timeStamps:relative}"
		s := withHeader(t, []string{header}, false,
			cue(timecode.MaxValidTime-2*time.Second-333*ms, timecode.MaxValidTime-time.Second-111*ms, "A"),
			cue(timecode.MaxValidTime-time.Second, timecode.MaxValidTime, "B"))
		_, _, err := read(write(t, s, model.CommonWriteOptions{}), model.CommonReadOptions{})
		require.NoError(t, err, scale)
	}
}

// The output reads back within half a unit for each time, also for relative timestamps.
func TestWriteReadsBackWithinHalfAUnit(t *testing.T) {
	t.Parallel()

	for _, scale := range []int{1, 10, 24, 25, 30, 600, 1000, 48000} {
		for _, relative := range []bool{false, true} {
			header := "{QTtext}{timeScale:" + strconv.Itoa(scale) + "}"
			tolerance := time.Second/time.Duration(scale)/2 + ms
			if relative {
				header += "{timeStamps:relative}"
			}
			s := withHeader(t, []string{header}, false)
			for i := range 40 {
				start := time.Duration(i*7919+13) * ms
				s.AddCues(cue(start, start+time.Duration(i*104729%7000+1)*ms, "x"))
			}
			back, _, err := read(write(t, s, model.CommonWriteOptions{}), model.CommonReadOptions{})
			require.NoError(t, err)
			require.Equal(t, s.Len(), back.Len())
			for i, c := range s.All() {
				got, _ := back.Cue(i)
				assert.InDelta(t, float64(c.Start), float64(got.Start), float64(tolerance), "scale %d relative %v cue %d start", scale, relative, i)
				assert.InDelta(t, float64(c.End), float64(got.End), float64(tolerance), "scale %d relative %v cue %d end", scale, relative, i)
			}
		}
	}
}

func TestWriteRelativeTimestampsTakeTheDifferenceFromTheReader(t *testing.T) {
	t.Parallel()

	t.Run("an overlap moves no later cue", func(t *testing.T) {
		t.Parallel()

		s := withHeader(t, []string{"{QTtext}{timeScale:1000}{timeStamps:relative}"}, false,
			cue(0, 5*time.Second, "A"), cue(3*time.Second, 6*time.Second, "B"), cue(10*time.Second, 12*time.Second, "C"))
		back, _, err := read(write(t, s, model.CommonWriteOptions{}), model.CommonReadOptions{})
		require.NoError(t, err)
		assert.Equal(t, []cueRow{{0, 5 * time.Second, "A"}, {5 * time.Second, 6 * time.Second, "B"}, {10 * time.Second, 12 * time.Second, "C"}}, cueRows(back))
	})
	t.Run("rounding does not add up", func(t *testing.T) {
		t.Parallel()

		s := withHeader(t, []string{"{QTtext}{timeScale:1}{timeStamps:relative}"}, false)
		for i := range 100 {
			start := time.Duration(i) * 1020 * ms
			s.AddCues(cue(start, start+1020*ms, "x"))
		}
		back, _, err := read(write(t, s, model.CommonWriteOptions{}), model.CommonReadOptions{})
		require.NoError(t, err)
		last, _ := back.Cue(99)
		want, _ := s.Cue(99)
		assert.InDelta(t, float64(want.End), float64(last.End), float64(time.Second))
	})
}

func TestWriteReturnsTheWriterError(t *testing.T) {
	t.Parallel()

	err := qttext.Write(errWriter{}, subtitle(cue(0, time.Second, "A")), model.CommonWriteOptions{}, qttext.WriteOptions{})
	require.Error(t, err)
	assert.ErrorIs(t, err, errWrite)
}
