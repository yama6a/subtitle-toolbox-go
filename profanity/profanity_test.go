package profanity_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/profanity"
)

func sec(s float64) time.Duration {
	return time.Duration(s * float64(time.Second)).Round(time.Millisecond)
}

func span(start, end float64) profanity.MuteRange {
	return profanity.MuteRange{Start: sec(start), End: sec(end)}
}

// cuesAt gives one cue per text, the cue i from times[i][0] to times[i][1] seconds.
func cuesAt(times [][2]float64, texts ...string) *model.Subtitle {
	var s model.Subtitle
	for i, text := range texts {
		c := model.Cue{Start: sec(times[i][0]), End: sec(times[i][1])}
		c.SetText(text)
		s.AddCue(c)
	}
	return &s
}

func load(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, rel), stbox.ReadOptions{})
	require.NoError(t, err)
	return s
}

func lines(s *model.Subtitle) [][]string {
	var out [][]string
	for _, c := range s.All() {
		out = append(out, c.Lines)
	}
	return out
}

// Ported from testMasks and maskCases of tests/Profanity/ProfanityFilterTest.php of subtitle-toolbox-php.
func TestApplyMasks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		line  string
		opts  profanity.Options
		want  string
	}{
		{"stars", []string{"hell"}, "What the hell?", profanity.Options{Mask: profanity.MaskStars}, "What the ****?"},
		{"zero mask is stars", []string{"hell"}, "What the hell?", profanity.Options{}, "What the ****?"},
		{"first letter", []string{"hell"}, "What the hell?", profanity.Options{Mask: profanity.MaskFirstLetter}, "What the h***?"},
		{"first letter keeps case", []string{"hell"}, "HELL no", profanity.Options{Mask: profanity.MaskFirstLetter}, "H*** no"},
		{"remove", []string{"hell"}, "What the hell is this?", profanity.Options{Mask: profanity.MaskRemove}, "What the is this?"},
		{"remove before a question mark", []string{"hell"}, "What the hell?", profanity.Options{Mask: profanity.MaskRemove}, "What the ?"},
		{"none", []string{"hell"}, "What the hell?", profanity.Options{Mask: profanity.MaskNone}, "What the hell?"},
		{"callback", []string{"hell"}, "What the hell?", profanity.Options{MaskFunc: func(w string) string { return "[" + w + "]" }}, "What the [hell]?"},
		{"callback with markup", []string{"hell"}, "What the hell?", profanity.Options{MaskFunc: func(w string) string { return "<" + w + ">" }}, "What the &lt;hell&gt;?"},
		{"callback wins over mask", []string{"hell"}, "What the hell?", profanity.Options{Mask: profanity.MaskNone, MaskFunc: func(string) string { return "beep" }}, "What the beep?"},
		{"case-insensitive", []string{"HeLL"}, "hell and Hell", profanity.Options{}, "**** and ****"},
		{"whole word only", []string{"hell"}, "Hello, shell, hell's", profanity.Options{}, "Hello, shell, ****'s"},
		{"wildcard ending", []string{"damn*"}, "Damn, damned, damnit, condemn", profanity.Options{}, "****, ******, ******, condemn"},
		{"unicode letters", []string{"h\u00f6lle", "schei\u00df*"}, "Zur H\u00d6LLE, Schei\u00dfkerl!", profanity.Options{Mask: profanity.MaskFirstLetter}, "Zur H****, S*********!"},
		{"unicode word boundary", []string{"hell"}, "Bellhell\u00e9 hell", profanity.Options{}, "Bellhell\u00e9 ****"},
		{"combining mark counted", []string{"cafe\u0301"}, "Le cafe\u0301", profanity.Options{}, "Le ****"},
		{"phrase", []string{"son of a"}, "You son of a gun", profanity.Options{}, "You ******** gun"},
		{"longest word first", []string{"son", "son of a"}, "You son of a gun", profanity.Options{}, "You ******** gun"},
		{"duplicate words", []string{"hell", "hell"}, "What the hell?", profanity.Options{}, "What the ****?"},
		{"inside tags", []string{"hell"}, "<i>What the hell</i>", profanity.Options{}, "<i>What the ****</i>"},
		{"entities stay escaped", []string{"hell"}, "hell &amp; &lt;b&gt;", profanity.Options{}, "**** &amp; &lt;b&gt;"},
		{"metacharacters are text", []string{"a.b", "(x)"}, "axb a.b (x)", profanity.Options{}, "axb *** ***"},
		{"every escaped metacharacter", []string{`x\.+?()|[]{}^$# y`}, `x\.+?()|[]{}^$# y z`, profanity.Options{}, "***************** z"},
		{"title case DZ", []string{"\u01c5"}, "\u01c4 \u01c6", profanity.Options{}, "* *"},
		{"title case LJ", []string{"\u01c8"}, "\u01c7", profanity.Options{}, "*"},
		{"Greek title case with ypogegrammeni", []string{"\u1f88"}, "\u1f80", profanity.Options{}, "*"},
		{"combining ypogegrammeni", []string{"\u0345"}, "\u03b9 \u0399", profanity.Options{}, "* *"},
		{"circled letter upper to lower", []string{"\u24b6"}, "\u24d0", profanity.Options{}, "*"},
		{"circled letter lower to upper", []string{"\u24d0"}, "\u24b6", profanity.Options{}, "*"},
		{"Roman numeral upper to lower", []string{"\u2160"}, "\u2170", profanity.Options{}, "*"},
		{"Roman numeral lower to upper", []string{"\u2170"}, "\u2160", profanity.Options{}, "*"},
		{"Kelvin sign and long s", []string{"ks"}, "\u212a\u017f KS", profanity.Options{}, "** **"},
		{"Greek final sigma", []string{"\u03c3\u03bf\u03c6\u03bf\u03c2"}, "\u03a3\u039f\u03a6\u039f\u03a3!", profanity.Options{}, "*****!"},
		{"format character", []string{"a\u0600b"}, "x a\u0600b", profanity.Options{}, "x **"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := cuesAt([][2]float64{{1, 2}}, tt.line)
			opts := tt.opts
			opts.Words = tt.words
			report, err := profanity.Apply(s, opts)
			require.NoError(t, err)
			assert.Equal(t, [][]string{{tt.want}}, lines(s))
			assert.Equal(t, []profanity.MuteRange{span(1, 2)}, report.MuteRanges)
		})
	}
}

// Ported from testNoMatchChangesNothing, testRemoveDeletesACueWithoutOtherText, testWordTimestampsSetTheRange,
// testRangesThatTouchOrOverlapJoin, testPaddingJoinsRangesAndStopsAtZero and testCuesWithoutTextAreSkipped of
// tests/Profanity/ProfanityFilterTest.php of subtitle-toolbox-php.
func TestApplyRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		s     *model.Subtitle
		opts  profanity.Options
		lines [][]string
		want  []profanity.MuteRange
	}{
		{
			name:  "no match changes nothing",
			s:     cuesAt([][2]float64{{1, 2}}, "Hello there"),
			opts:  profanity.Options{Words: []string{"hell"}},
			lines: [][]string{{"Hello there"}},
		},
		{
			name:  "remove deletes a cue without other text",
			s:     cuesAt([][2]float64{{1, 2}, {3, 4}}, "<i>Damn</i>", "Fine."),
			opts:  profanity.Options{Words: []string{"damn"}, Mask: profanity.MaskRemove},
			lines: [][]string{{"Fine."}},
			want:  []profanity.MuteRange{span(1, 2)},
		},
		{
			name: "word timestamps set the range",
			s: cuesAt([][2]float64{{10, 14}},
				"Damn <00:00:10.500>this <00:00:11.000>hell\n<00:00:12.000>is <00:00:12.500>hot."),
			opts: profanity.Options{Words: []string{"damn", "hell", "hot"}},
			lines: [][]string{{
				"**** <00:00:10.500>this <00:00:11.000>****",
				"<00:00:12.000>is <00:00:12.500>***.",
			}},
			want: []profanity.MuteRange{span(10, 10.5), span(11, 12), span(12.5, 14)},
		},
		{
			name:  "ranges that touch or overlap join",
			s:     cuesAt([][2]float64{{1, 2}, {2, 3}, {2.5, 4}, {5, 6}, {6.3, 7}}, "hell", "hell", "hell", "hell", "hell"),
			opts:  profanity.Options{Words: []string{"hell"}},
			lines: [][]string{{"****"}, {"****"}, {"****"}, {"****"}, {"****"}},
			want:  []profanity.MuteRange{span(1, 4), span(5, 6), span(6.3, 7)},
		},
		{
			name:  "padding joins ranges and stops at zero",
			s:     cuesAt([][2]float64{{0.1, 1}, {1.3, 2}}, "hell", "hell"),
			opts:  profanity.Options{Words: []string{"hell"}, Padding: 200 * time.Millisecond},
			lines: [][]string{{"****"}, {"****"}},
			want:  []profanity.MuteRange{span(0, 2.2)},
		},
		{
			name:  "several matches in a run give one range",
			s:     cuesAt([][2]float64{{1, 2}}, "hell and hell"),
			opts:  profanity.Options{Words: []string{"hell"}, Mask: profanity.MaskNone},
			lines: [][]string{{"hell and hell"}},
			want:  []profanity.MuteRange{span(1, 2)},
		},
		{
			name:  "word timestamp after the cue end",
			s:     cuesAt([][2]float64{{10, 13.2}}, "<00:00:10.000>What <00:00:13.500>hell"),
			opts:  profanity.Options{Words: []string{"hell"}},
			lines: [][]string{{"<00:00:10.000>What <00:00:13.500>****"}},
			want:  []profanity.MuteRange{span(13.2, 13.5)},
		},
		{
			name:  "invalid UTF-8 run is skipped",
			s:     cuesAt([][2]float64{{1, 2}}, "hell \xff<i>hell</i>"),
			opts:  profanity.Options{Words: []string{"hell"}},
			lines: [][]string{{"hell \xff<i>****</i>"}},
			want:  []profanity.MuteRange{span(1, 2)},
		},
		{
			name:  "cues without lines are skipped",
			s:     cuesAt([][2]float64{{1, 2}}, ""),
			opts:  profanity.Options{Words: []string{"hell"}},
			lines: [][]string{nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report, err := profanity.Apply(tt.s, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.lines, lines(tt.s))
			assert.Equal(t, tt.want, report.MuteRanges)
		})
	}
}

func TestApplyMovesTheCommentsOfARemovedCue(t *testing.T) {
	t.Parallel()

	s := cuesAt([][2]float64{{1, 2}, {3, 4}}, "Damn", "Fine.")
	require.NoError(t, s.AddComment("before damn", 0))
	require.NoError(t, s.AddComment("before fine", 1))
	_, err := profanity.Apply(s, profanity.Options{Words: []string{"damn"}, Mask: profanity.MaskRemove})
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"Fine."}}, lines(s))
	assert.Equal(t, []model.Comment{{Text: "before damn", BeforeCue: 0}, {Text: "before fine", BeforeCue: 0}}, s.Comments())
}

// Ported from testRealSubRipFileMatchesTheLastCue, testRealSubRipFileWithPaddingJoinsTheLastRanges and
// testRealWebVttFileWithWordTimestamps of tests/Profanity/ProfanityFilterTest.php of subtitle-toolbox-php.
func TestApplyRealFiles(t *testing.T) {
	t.Parallel()

	words := []string{"damn*", "hell"}
	t.Run("SubRip", func(t *testing.T) {
		t.Parallel()

		s := load(t, "profanity/keys.srt")
		report, err := profanity.Apply(s, profanity.Options{Words: words})
		require.NoError(t, err)
		keys := []profanity.MuteRange{span(3.4, 5), span(8, 9.1), span(11.5, 13), span(13.2, 15.6)}
		assert.Equal(t, keys, report.MuteRanges)
		require.Equal(t, 7, s.Len())
		got := lines(s)
		assert.Equal(t, []string{"Where did I put the keys?"}, got[0])
		assert.Equal(t, []string{"- Hello?", "- Go to ****."}, got[5])
		assert.Equal(t, []string{"That ****** door again."}, got[6])
		last, _ := s.Cue(6)
		assert.Equal(t, [2]time.Duration{sec(13.2), sec(15.6)}, [2]time.Duration{last.Start, last.End})

		var buf bytes.Buffer
		require.NoError(t, stbox.Write(&buf, s, stbox.SubRip, stbox.WriteOptions{LineEnding: stbox.CRLF, BOM: stbox.BOMRemove}))
		want := strings.NewReplacer("Damn it", "**** it", "the hell", "the ****", "to hell", "to ****", "damned", "******").
			Replace(string(testfiles.Read(t, "profanity/keys.srt")))
		assert.Equal(t, want, buf.String())

		report, err = profanity.Apply(load(t, "profanity/keys.srt"), profanity.Options{Words: words, Mask: profanity.MaskNone})
		require.NoError(t, err)
		assert.Equal(t, keys, report.MuteRanges)
	})
	t.Run("SubRip with padding", func(t *testing.T) {
		t.Parallel()

		report, err := profanity.Apply(load(t, "profanity/keys.srt"), profanity.Options{Words: words, Padding: 250 * time.Millisecond})
		require.NoError(t, err)
		assert.Equal(t, []profanity.MuteRange{span(3.15, 5.25), span(7.75, 9.35), span(11.25, 15.85)}, report.MuteRanges)
		assert.Equal(t, "3.150 5.250 1\n7.750 9.350 1\n11.250 15.850 1\n", profanity.EDL(report.MuteRanges))
	})
	t.Run("WebVTT", func(t *testing.T) {
		t.Parallel()

		s := load(t, "profanity/radio.vtt")
		report, err := profanity.Apply(s, profanity.Options{Words: words, Mask: profanity.MaskFirstLetter})
		require.NoError(t, err)
		require.Equal(t, 4, s.Len())
		assert.Equal(t, []profanity.MuteRange{span(1.6, 2), span(6.3, 6.7), span(8, 9)}, report.MuteRanges)
		got := lines(s)
		assert.Equal(t, []string{"<00:00:01.000>Turn <00:00:01.400>the <00:00:01.600>d*** <00:00:02.000>radio <00:00:02.500>off."}, got[0])
		assert.Equal(t, []string{"<00:00:07.500>Fine, <00:00:08.000>d*****."}, got[3])

		var buf bytes.Buffer
		require.NoError(t, stbox.Write(&buf, s, stbox.WebVTT, stbox.WriteOptions{BOM: stbox.BOMRemove}))
		want := strings.NewReplacer("damn <", "d*** <", "hell <", "h*** <", "damnit.", "d*****.").
			Replace(string(testfiles.Read(t, "profanity/radio.vtt")))
		assert.Equal(t, want, buf.String())
		assert.Equal(t, "volume=enable='between(t,1.600,2.000)+between(t,6.300,6.700)+between(t,8.000,9.000)':volume=0",
			profanity.FFmpegVolumeFilter(report.MuteRanges))
	})
}

func TestApplyMatchesEachCharacterAsItself(t *testing.T) {
	t.Parallel()

	runes := []rune{'\u00a0', '\u00ad', '\u0600', '\u200b', '\u2028', '\ufeff', '\ufffd', '\U0001d173', '\U0010ffff'}
	for r := rune(0x20); r < 0x7f; r++ {
		if r != '*' {
			runes = append(runes, r)
		}
	}
	escape := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	for _, r := range runes {
		word := "x" + string(r) + "y"
		s := cuesAt([][2]float64{{1, 2}}, escape.Replace(word))
		report, err := profanity.Apply(s, profanity.Options{Words: []string{word}})
		require.NoError(t, err, "%U", r)
		assert.Len(t, report.MuteRanges, 1, "%U", r)
	}
}

// Ported from testMuteRangesUseTheShiftedWordTimestamps of tests/WordTimestampRetimingTest.php of
// subtitle-toolbox-php.
func TestApplyUsesTheShiftedWordTimestamps(t *testing.T) {
	t.Parallel()

	s := load(t, "profanity/radio.vtt")
	require.NoError(t, s.Shift(100*time.Second))
	report, err := profanity.Apply(s, profanity.Options{Words: []string{"damn"}})
	require.NoError(t, err)
	assert.Equal(t, []profanity.MuteRange{span(101.6, 102)}, report.MuteRanges)
}

// Ported from testFirstLetterMaskKeepsWordTimestampsAndPadsTheMuteRange and testEmptyRangeListWritesEmptyStrings of
// tests/Profanity/ProfanityFilterTest.php of subtitle-toolbox-php.
func TestEDLAndFFmpegVolumeFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ranges []profanity.MuteRange
		edl    string
		filter string
	}{
		{"no ranges", nil, "", ""},
		{"one range", []profanity.MuteRange{span(62.38, 62.9)}, "62.380 62.900 1\n", "volume=enable='between(t,62.380,62.900)':volume=0"},
		{
			"two ranges",
			[]profanity.MuteRange{span(0, 0.005), span(3600.05, 7200)},
			"0.000 0.005 1\n3600.050 7200.000 1\n",
			"volume=enable='between(t,0.000,0.005)+between(t,3600.050,7200.000)':volume=0",
		},
		{
			"negative and fractional milliseconds",
			[]profanity.MuteRange{{Start: -1500 * time.Millisecond, End: 1999500 * time.Microsecond}},
			"-1.500 2.000 1\n",
			"volume=enable='between(t,-1.500,2.000)':volume=0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.edl, profanity.EDL(tt.ranges))
			assert.Equal(t, tt.filter, profanity.FFmpegVolumeFilter(tt.ranges))
		})
	}
}

// Ported from testInvalidOptionsThrow and invalidOptions of tests/Profanity/ProfanityFilterTest.php of
// subtitle-toolbox-php.
func TestValidate(t *testing.T) {
	t.Parallel()

	bad := func(w string) string {
		return "A profanity word must be a non-empty UTF-8 string with a * only at the end, got " + w + "."
	}
	tests := []struct {
		name   string
		opts   profanity.Options
		option string
		msg    string
	}{
		{"empty list", profanity.Options{}, "Words", "The profanity word list is empty."},
		{"empty word", profanity.Options{Words: []string{""}}, "Words", bad("''")},
		{"star in the middle", profanity.Options{Words: []string{"f*ck"}}, "Words", bad("'f*ck'")},
		{"only a star", profanity.Options{Words: []string{"*"}}, "Words", bad("'*'")},
		{"two stars at the end", profanity.Options{Words: []string{"a**"}}, "Words", bad("'a**'")},
		{"star at the start", profanity.Options{Words: []string{"*a"}}, "Words", bad("'*a'")},
		{"space at the end", profanity.Options{Words: []string{"hell "}}, "Words", bad("'hell '")},
		{"space at the start", profanity.Options{Words: []string{" hell"}}, "Words", bad("' hell'")},
		{"space before the star", profanity.Options{Words: []string{"hell *"}}, "Words", bad("'hell *'")},
		{"line feed at the end", profanity.Options{Words: []string{"hell\n"}}, "Words", bad("'hell\n'")},
		{"no-break space at the end", profanity.Options{Words: []string{"hell\u00a0"}}, "Words", bad("'hell\u00a0'")},
		{"Mongolian vowel separator at the start", profanity.Options{Words: []string{"\u180ehell"}}, "Words", bad("'\u180ehell'")},
		{"quote and backslash", profanity.Options{Words: []string{`it's\ `}}, "Words", bad(`'it\'s\\ '`)},
		{"invalid UTF-8", profanity.Options{Words: []string{"h\xe9ll"}}, "Words", bad("'h\xe9ll'")},
		{"bad word before empty list", profanity.Options{Words: []string{"ok", "*"}, Padding: -1}, "Words", bad("'*'")},
		{"negative padding", profanity.Options{Words: []string{"hell"}, Padding: -100 * time.Millisecond}, "Padding", "The padding must not be negative, got -0.1."},
		{"unknown mask", profanity.Options{Words: []string{"hell"}, Mask: "firstLetter"}, "Mask", `The mask "firstLetter" is not stars, first-letter, remove or none.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var aerr *model.InvalidArgumentError
			require.ErrorAs(t, err, &aerr)
			assert.Equal(t, "profanity.Options."+tt.option, aerr.Option)
			assert.Equal(t, tt.msg, aerr.Msg)

			s := cuesAt([][2]float64{{1, 2}}, "hell")
			_, applyErr := profanity.Apply(s, tt.opts)
			assert.Equal(t, err, applyErr)
			assert.Equal(t, [][]string{{"hell"}}, lines(s))
		})
	}

	for _, opts := range []profanity.Options{
		{Words: []string{"a"}},
		{Words: []string{"a b*", "x\u00a0y", "\u00e9t\u00e9"}, Mask: profanity.MaskNone, Padding: time.Second},
		{Words: []string{"a"}, Mask: profanity.MaskFirstLetter},
		{Words: []string{"a"}, Mask: profanity.MaskRemove},
		{Words: []string{"a"}, Mask: profanity.MaskStars},
	} {
		assert.NoError(t, opts.Validate(), opts.Words)
	}
}

func TestApplyLongPadding(t *testing.T) {
	t.Parallel()

	s := cuesAt([][2]float64{{1, 2}}, "hell")
	report, err := profanity.Apply(s, profanity.Options{Words: []string{"hell"}, Padding: time.Duration(1<<63 - 1)})
	require.NoError(t, err)
	require.Len(t, report.MuteRanges, 1)
	assert.Equal(t, time.Duration(0), report.MuteRanges[0].Start)
	assert.Greater(t, report.MuteRanges[0].End, time.Duration(0))
}
