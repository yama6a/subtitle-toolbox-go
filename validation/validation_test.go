package validation_test

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/stats"
	"github.com/yama6a/subtitle-toolbox-go/v3/validation"
)

// cue builds a cue from times in milliseconds.
func cue(startMS, endMS int, lines ...string) model.Cue {
	return model.Cue{Start: time.Duration(startMS) * time.Millisecond, End: time.Duration(endMS) * time.Millisecond, Lines: lines}
}

func subtitle(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func load(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, rel), stbox.ReadOptions{})
	require.NoError(t, err)
	return s
}

func number(v float64) string {
	if math.IsInf(v, 1) {
		return "+Inf"
	}
	return strconv.FormatFloat(math.Round(v*1e6)/1e6, 'f', -1, 64)
}

// lines formats each violation as "CUE RULE VALUE" and ", limit LIMIT" when it has a limit. The numbers are rounded
// to 6 decimals.
func lines(t *testing.T, s *model.Subtitle, r validation.Rules) []string {
	t.Helper()

	got, err := validation.Check(s, r)
	require.NoError(t, err)
	var out []string //nolint:prealloc // an empty result must stay nil for the table
	for _, v := range got {
		line := fmt.Sprintf("%d %s %s", v.Cue, v.Rule, number(v.Value))
		if v.Limit != nil {
			line += ", limit " + number(*v.Limit)
		}
		out = append(out, line)
	}
	return out
}

func style(s model.DialogueDashStyle) *model.DialogueDashStyle { return &s }

type row struct {
	name  string
	cues  []model.Cue
	rules validation.Rules
	want  []string
}

func runRows(t *testing.T, rows []row) {
	t.Helper()

	for _, tt := range rows {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, lines(t, subtitle(tt.cues...), tt.rules))
		})
	}
}

func TestTimeRules(t *testing.T) {
	t.Parallel()

	third := 5 * time.Second / 6
	runRows(t, []row{
		{"no rules", []model.Cue{cue(0, 1, "x"), cue(0, 1, "")}, validation.Rules{}, nil},
		{"no cues", nil, validation.Rules{RequireCues: true}, []string{"-1 requireCues 0"}},
		{"negative duration", []model.Cue{cue(2000, 1500, "x")}, validation.Rules{NoNegativeDuration: true}, []string{"0 noNegativeDuration -0.5"}},
		{
			"limits meet cue times in milliseconds",
			[]model.Cue{cue(0, 833, "a"), cue(916, 1749, "b"), cue(1831, 2664, "c")},
			validation.Rules{MinDuration: third, MinGap: 2 * time.Second / 24},
			[]string{"2 minGap 0.082, limit 0.083333"},
		},
		{
			"minimum and maximum duration",
			[]model.Cue{cue(0, 400, "a"), cue(1000, 8500, "b"), cue(9000, 10000, "c")},
			validation.Rules{MinDuration: third, MaxDuration: 7 * time.Second},
			[]string{"0 minDuration 0.4, limit 0.833333", "1 maxDuration 7.5, limit 7"},
		},
		{
			"overlap uses the latest end of all earlier cues",
			[]model.Cue{cue(0, 10000, "long"), cue(2000, 3000, "short"), cue(5000, 6000, "inside"), cue(10000, 11000, "after")},
			validation.Rules{NoOverlap: true},
			[]string{"1 noOverlap 8", "2 noOverlap 5"},
		},
		{
			"the gap rule skips overlaps",
			[]model.Cue{cue(0, 2000, "a"), cue(1000, 3000, "b"), cue(3000, 4000, "c")},
			validation.Rules{MinGap: 500 * time.Millisecond},
			[]string{"2 minGap 0, limit 0.5"},
		},
		{
			"zero duration reads at infinite speed",
			[]model.Cue{cue(5000, 5000, "Hi"), cue(6000, 6000, "")},
			validation.Rules{MaxCharactersPerSecond: 20},
			[]string{"0 maxCharactersPerSecond +Inf, limit 20"},
		},
		{
			"an infinite limit checks nothing",
			[]model.Cue{cue(5000, 5000, "Hi there")},
			validation.Rules{MaxCharactersPerSecond: math.Inf(1), MaxWordsPerMinute: math.Inf(1)},
			nil,
		},
		{
			"minimum per word at the exact limit",
			[]model.Cue{cue(0, 1001, "word"), cue(2000, 3000, "word")},
			validation.Rules{MinDurationPerWord: 1001 * time.Millisecond},
			[]string{"1 minSecondsPerWord 1, limit 1.001"},
		},
		{
			"word rules skip cues without words and match milliseconds",
			[]model.Cue{cue(0, 1200, "Four words right here"), cue(2000, 2000, "Now"), cue(3000, 4000, ""), cue(5000, 6199, "Four words too fast")},
			validation.Rules{MaxWordsPerMinute: 200, MinDurationPerWord: 300 * time.Millisecond},
			[]string{
				"1 maxWordsPerMinute +Inf, limit 200", "1 minSecondsPerWord 0, limit 0.3",
				"3 maxWordsPerMinute 200.166806, limit 200", "3 minSecondsPerWord 0.29975, limit 0.3",
			},
		},
	})
}

func TestCountRules(t *testing.T) {
	t.Parallel()

	runRows(t, []row{
		{"empty cues", []model.Cue{cue(0, 1, "x"), cue(1, 2), cue(2, 3, "<i> </i>")}, validation.Rules{NoEmptyCues: true}, []string{"1 noEmptyCues 0", "2 noEmptyCues 0"}},
		{
			"markup and multibyte letters",
			[]model.Cue{cue(0, 10000, "<b>Gr\u00f6\u00dfe: zw\u00f6lf \u00c4pfel, drei\u00dfig Birnen, sechs.</b>", "<i>Fish &amp; chips</i>")},
			validation.Rules{MaxCharactersPerLine: 41, MaxCharactersPerSecond: 5},
			[]string{"0 maxCharactersPerLine 42, limit 41", "0 maxCharactersPerSecond 5.4, limit 5"},
		},
		{
			"the limit is allowed",
			[]model.Cue{cue(0, 10000, "<b>Gr\u00f6\u00dfe: zw\u00f6lf \u00c4pfel, drei\u00dfig Birnen, sechs.</b>")},
			validation.Rules{MaxCharactersPerLine: 42},
			nil,
		},
		{"bytes of invalid UTF-8", []model.Cue{cue(0, 1000, "a\xff\xfe")}, validation.Rules{MaxCharactersPerLine: 2}, []string{"0 maxCharactersPerLine 3, limit 2"}},
		{"lines with only markup do not count", []model.Cue{cue(0, 1000, "One", "<i> </i>", "Two")}, validation.Rules{MaxLinesPerCue: 2}, nil},
		{"3 lines", []model.Cue{cue(0, 1000, "One", "Two", "Three")}, validation.Rules{MaxLinesPerCue: 2}, []string{"0 maxLinesPerCue 3, limit 2"}},
		{
			"speakers from voices",
			[]model.Cue{cue(0, 1000, "<v Anna>Hi", "<v.loud Tom>Hello", "<v Anna>Bye"), cue(1000, 2000, "<v Anna>Hi", "Bye")},
			validation.Rules{MaxSpeakersPerCue: 1},
			[]string{"0 maxSpeakersPerCue 2, limit 1"},
		},
		{
			"speakers from dashes",
			[]model.Cue{cue(0, 1000, "- Hi", "- Hello", "- Bye"), cue(1000, 2000, "-20 degrees", "text"), cue(2000, 3000)},
			validation.Rules{MaxSpeakersPerCue: 2},
			[]string{"0 maxSpeakersPerCue 3, limit 2"},
		},
	})
}

func TestTextRules(t *testing.T) {
	t.Parallel()

	broadcast := "[A-Za-z0-9!)(,.?:\\-><&@#%+*=/\u00a3$\u00a2\u00a5\u00a9\u00ae\u00bc\u00bd\u00be\u2122'\"]"
	hyphenSpace, hyphen := style(model.DialogueDashHyphenSpace), style(model.DialogueDashHyphen)
	tags := func(name string, want int, lines ...string) row {
		var w []string
		if want > 0 {
			w = []string{fmt.Sprintf("0 noUnbalancedTags %d", want)}
		}
		return row{"tags " + name, []model.Cue{cue(0, 1000, lines...)}, validation.Rules{NoUnbalancedTags: true}, w}
	}
	runRows(t, []row{
		tags("voice over 2 lines", 0, "<v Anna><i>Over two", "lines</i>"),
		tags("crossed", 2, "<b><i>Crossed</b></i>"),
		tags("stray and open", 2, "Stray</u> and <font color=\"#ff0000\">open"),
		tags("other tags", 0, "<c.red>Other</c> tags and <00:00:03.500>word timestamps"),
		tags("escaped", 0, "Text &lt;i&gt; that looks like a tag"),
		tags("2 rt", 2, "<ruby>a<rt>b<rt>c"),
		tags("rt closed by ruby", 1, "<ruby>a<rt><i>b</ruby>"),
		tags("rt outside ruby", 3, "<b>a<rt>b</rt></b>", "<rt>c"),
		tags("ruby over 2 lines", 0, "<ruby>a", "<rt>b</rt></ruby>"),
		tags("upper case", 1, "<B>a"),
		tags("br", 0, "a<br>b"),
		{
			"spaces",
			[]model.Cue{
				cue(0, 1000, "One <i> two</i>", "<b>three </b>"),
				cue(1000, 2000, "Four\u00a0\u00a0five", "\u00a0six"),
				cue(2000, 3000, "No <i>problem</i> here"),
			},
			validation.Rules{NoDoubleSpaces: true, NoLeadingOrTrailingSpaces: true},
			[]string{"0 noDoubleSpaces 1", "0 noLeadingOrTrailingSpaces 1", "1 noDoubleSpaces 1", "1 noLeadingOrTrailingSpaces 1"},
		},
		{
			"dash style with a space",
			[]model.Cue{cue(0, 1000, "- Yes.", "-No."), cue(1000, 2000, "<i>-Maybe.</i>", "\u2014Never."), cue(2000, 3000, "-20 degrees.", "--- a line", "- ")},
			validation.Rules{DialogueDashStyle: hyphenSpace},
			[]string{"0 dialogueDashStyle 1", "1 dialogueDashStyle 2"},
		},
		{
			"dash style without a space",
			[]model.Cue{cue(0, 1000, "- Yes.", "-No."), cue(1000, 2000, "<i>-Maybe.</i>", "\u2014Never.")},
			validation.Rules{DialogueDashStyle: hyphen},
			[]string{"0 dialogueDashStyle 1", "1 dialogueDashStyle 1"},
		},
		{
			"dash after a bidi control",
			[]model.Cue{cue(0, 1000, "\u202b-No.")},
			validation.Rules{DialogueDashStyle: hyphenSpace},
			[]string{"0 dialogueDashStyle 1"},
		},
		{
			"allowed list",
			[]model.Cue{cue(0, 1000, "Caf\u00e9 a/b [c]")},
			validation.Rules{AllowedCharacters: "Cafab"},
			[]string{"0 allowedCharacters 5"},
		},
		{"allowed class", []model.Cue{cue(0, 1000, "a/b")}, validation.Rules{AllowedCharacters: "[a-z/]"}, nil},
		{
			"allowed class on text with accents",
			[]model.Cue{cue(0, 1000, "The caf\u00e9 sells cr\u00e8me for \u00a34 &amp; tea")},
			validation.Rules{AllowedCharacters: broadcast},
			[]string{"0 allowedCharacters 2"},
		},
		{
			"invalid UTF-8",
			[]model.Cue{cue(0, 1000, "CAF\xc9 \xc9T\xc9")},
			validation.Rules{NoDoubleSpaces: true, AllowedCharacters: "[A-Za-z]", NoAllCapsLines: true},
			[]string{"0 allowedCharacters 3", "0 noAllCapsLines 1"},
		},
		{
			"all caps",
			[]model.Cue{
				cue(0, 1000, "<v ANNA>Hello.", "[BELL RINGS]", "(SHOUTS) Stop!", "I"),
				cue(1000, 2000, "STOP THE TRAIN!", "<i>NOW</i>", "\u00c4RGER"),
				cue(2000, 3000, "(SHOUTING) STOP"),
			},
			validation.Rules{NoAllCapsLines: true},
			[]string{"1 noAllCapsLines 3", "2 noAllCapsLines 1"},
		},
		{
			"one cue breaks 6 rules in order",
			[]model.Cue{cue(10000, 11000, "-Where are you?\u00a0 <i>Home", "- Wait.", "- Now!")},
			validation.Rules{
				NoDoubleSpaces: true, NoUnbalancedTags: true, DialogueDashStyle: hyphenSpace, MaxSpeakersPerCue: 2,
				MaxWordsPerMinute: 180, MinDurationPerWord: 300 * time.Millisecond,
			},
			[]string{
				"0 noDoubleSpaces 1", "0 noUnbalancedTags 1", "0 dialogueDashStyle 1", "0 maxSpeakersPerCue 3, limit 2",
				"0 maxWordsPerMinute 480, limit 180", "0 minSecondsPerWord 0.125, limit 0.3",
			},
		},
	})
}

func TestEveryRuleInOrder(t *testing.T) {
	t.Parallel()

	rules := validation.Rules{
		MaxCharactersPerSecond: 1, MaxCharactersPerLine: 1, MaxLinesPerCue: 1, MinDuration: time.Minute, MaxDuration: time.Millisecond,
		MinGap: time.Minute, NoOverlap: true, NoEmptyCues: true, NoDoubleSpaces: true, NoLeadingOrTrailingSpaces: true,
		NoUnbalancedTags: true, DialogueDashStyle: style(model.DialogueDashEnDash), MaxSpeakersPerCue: 1, MaxWordsPerMinute: 1,
		MinDurationPerWord: time.Minute, AllowedCharacters: "x", NoAllCapsLines: true, RequireCues: true, NoNegativeDuration: true,
	}
	assert.Equal(t, []string{"-1 requireCues 0"}, lines(t, subtitle(), rules)[:1])

	got := lines(t, subtitle(cue(0, 2000, "x"), cue(500, 400, "- A\u00a0\u00a0B", "<b>- C"), cue(600, 700, "")), rules)
	var names []string
	for _, l := range got {
		if strings.HasPrefix(l, "1 ") {
			names = append(names, strings.Fields(l)[1])
		}
	}
	assert.Equal(t, []string{
		"noNegativeDuration", "maxCharactersPerLine", "maxCharactersPerLine", "maxLinesPerCue", "maxCharactersPerSecond", "minDuration",
		"noDoubleSpaces", "noUnbalancedTags", "dialogueDashStyle", "maxSpeakersPerCue", "maxWordsPerMinute",
		"minSecondsPerWord", "allowedCharacters", "noAllCapsLines", "noOverlap",
	}, names)
}

func TestRulesValidate(t *testing.T) {
	t.Parallel()

	bad := model.DialogueDashStyle(9)
	tests := []struct {
		name  string
		rules validation.Rules
		opt   string
		msg   string
	}{
		{"negative cps", validation.Rules{MaxCharactersPerSecond: -1}, "Rules.MaxCharactersPerSecond", "The limit maxCharactersPerSecond must be 0 or more, got -1."},
		{"NaN wpm", validation.Rules{MaxWordsPerMinute: math.NaN()}, "Rules.MaxWordsPerMinute", "The limit maxWordsPerMinute must be 0 or more, got NAN."},
		{"negative cpl", validation.Rules{MaxCharactersPerLine: -2}, "Rules.MaxCharactersPerLine", "The limit maxCharactersPerLine must be 0 or more, got -2."},
		{"negative lines", validation.Rules{MaxLinesPerCue: -1}, "Rules.MaxLinesPerCue", "The limit maxLinesPerCue must be 0 or more, got -1."},
		{"negative speakers", validation.Rules{MaxSpeakersPerCue: -1}, "Rules.MaxSpeakersPerCue", "The limit maxSpeakersPerCue must be 0 or more, got -1."},
		{"negative max duration", validation.Rules{MaxDuration: -1500 * time.Millisecond}, "Rules.MaxDuration", "The limit maxDuration must be 0 or more, got -1.5."},
		{"negative min duration", validation.Rules{MinDuration: -time.Second}, "Rules.MinDuration", "The limit minDuration must be 0 or more, got -1."},
		{"negative min gap", validation.Rules{MinGap: -time.Second}, "Rules.MinGap", "The limit minGap must be 0 or more, got -1."},
		{"negative duration per word", validation.Rules{MinDurationPerWord: -time.Second}, "Rules.MinDurationPerWord", "The limit minSecondsPerWord must be 0 or more, got -1."},
		{"unknown style", validation.Rules{DialogueDashStyle: &bad}, "Rules.DialogueDashStyle", "The dialogue dash style 9 is not one of the 8 styles."},
		{"bad class", validation.Rules{AllowedCharacters: "[z-a]"}, "Rules.AllowedCharacters", `The allowed characters "[z-a]" are not a valid character class: `},
		{
			"POSIX class",
			validation.Rules{AllowedCharacters: "[[:alpha:]]"},
			"Rules.AllowedCharacters",
			`The allowed characters "[[:alpha:]]" hold a POSIX class such as [:alpha:]. Use a Unicode class such as \p{L}.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.rules.Validate()
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var arg *model.InvalidArgumentError
			require.ErrorAs(t, err, &arg)
			assert.Equal(t, tt.opt, arg.Option)
			assert.True(t, strings.HasPrefix(arg.Msg, tt.msg), arg.Msg)
			_, checkErr := validation.Check(subtitle(), tt.rules)
			assert.Equal(t, err, checkErr)
		})
	}
	assert.NoError(t, validation.Rules{AllowedCharacters: "[a-z]", MaxCharactersPerSecond: math.Inf(1)}.Validate())
}

func TestCheckLeavesTheSubtitleAlone(t *testing.T) {
	t.Parallel()

	s := subtitle(cue(0, 1000, "<i>Hi", "there"), cue(500, 900, "CAPS  LOCK"))
	before := s.Cues()
	before[0].Lines = append([]string(nil), before[0].Lines...)
	_, err := validation.Check(s, validation.Rules{NoUnbalancedTags: true, NoAllCapsLines: true, NoOverlap: true, AllowedCharacters: "[a-z]"})
	require.NoError(t, err)
	assert.Equal(t, before, s.Cues())
}

func TestFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		file  string
		rules validation.Rules
		want  []string
	}{
		{"no rules", "validation/own_netflix_checks.srt", validation.Rules{}, nil},
		{"empty cues", "validation/own_netflix_checks.srt", validation.Rules{NoEmptyCues: true}, []string{"6 noEmptyCues 0"}},
		{
			"grapheme counts", "statistics/own_combining_marks.srt",
			validation.Rules{MaxCharactersPerLine: 31},
			[]string{"0 maxCharactersPerLine 32, limit 31", "4 maxCharactersPerLine 43, limit 31"},
		},
		{
			"dashes after bidi controls", "fixing/rtl-dialogue.srt",
			validation.Rules{MaxSpeakersPerCue: 1, DialogueDashStyle: style(model.DialogueDashHyphenSpace)},
			[]string{
				"0 dialogueDashStyle 2", "0 maxSpeakersPerCue 2, limit 1", "1 dialogueDashStyle 2", "1 maxSpeakersPerCue 2, limit 1",
				"2 dialogueDashStyle 2", "2 maxSpeakersPerCue 2, limit 1", "4 dialogueDashStyle 1", "4 maxSpeakersPerCue 2, limit 1",
			},
		},
		{
			"overlap in a real file", "srt/real/language_subtitles_dots_tester.srt",
			validation.Rules{MaxCharactersPerLine: 42, NoOverlap: true, MinGap: 2 * time.Second / 24},
			[]string{"0 maxCharactersPerLine 62, limit 42", "1 minGap 0.001, limit 0.083333", "2 noOverlap 1.999"},
		},
		{"real WebVTT within limits", "vtt/real/w3c_comments.vtt", validation.Rules{MaxCharactersPerLine: 42, NoOverlap: true}, nil},
		{"real WebVTT line length", "vtt/real/w3c_comments.vtt", validation.Rules{MaxCharactersPerLine: 30}, []string{"1 maxCharactersPerLine 33, limit 30"}},
		{"voices", "vtt/real/w3c_voices.vtt", validation.Rules{MaxSpeakersPerCue: 1}, nil},
		{"dashes in WebVTT", "vtt/real/webvttpy_netflix.vtt", validation.Rules{MaxSpeakersPerCue: 1}, []string{"17 maxSpeakersPerCue 2, limit 1"}},
		{"matching dashes", "vtt/real/webvttpy_netflix.vtt", validation.Rules{DialogueDashStyle: style(model.DialogueDashHyphenSpace)}, nil},
		{
			"other dashes", "vtt/real/webvttpy_netflix.vtt",
			validation.Rules{DialogueDashStyle: style(model.DialogueDashEnDashSpace)},
			[]string{"17 dialogueDashStyle 2"},
		},
		{"styled SubRip", "srt/real/own_styled.srt", validation.Rules{NoUnbalancedTags: true}, []string{"7 noUnbalancedTags 1"}},
		{
			"WebVTT tags", "validation/own_webvtt_tags.vtt",
			validation.Rules{NoUnbalancedTags: true},
			[]string{"0 noUnbalancedTags 1", "1 noUnbalancedTags 1", "3 noUnbalancedTags 1", "5 noUnbalancedTags 1", "8 noUnbalancedTags 1"},
		},
		{
			"text rules", "validation/own_text_checks.vtt",
			validation.Rules{
				NoDoubleSpaces: true, NoLeadingOrTrailingSpaces: true, NoUnbalancedTags: true, DialogueDashStyle: style(model.DialogueDashHyphenSpace),
				MaxSpeakersPerCue: 2, MaxWordsPerMinute: 180, MinDurationPerWord: 300 * time.Millisecond, NoAllCapsLines: true,
			},
			[]string{
				"0 noDoubleSpaces 1", "0 noUnbalancedTags 1", "0 dialogueDashStyle 1", "0 maxSpeakersPerCue 3, limit 2",
				"0 maxWordsPerMinute 480, limit 180", "0 minSecondsPerWord 0.125, limit 0.3",
				"1 noDoubleSpaces 1", "1 noLeadingOrTrailingSpaces 2", "3 noAllCapsLines 1", "4 dialogueDashStyle 2", "6 noUnbalancedTags 2",
			},
		},
		{"text rules off", "validation/own_text_checks.vtt", validation.Rules{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, lines(t, load(t, tt.file), tt.rules))
		})
	}
}

func TestRightToLeftKeepsTheCounts(t *testing.T) {
	t.Parallel()

	rules := validation.Rules{MaxCharactersPerSecond: 15, MaxCharactersPerLine: 10}
	assert.Equal(t, lines(t, load(t, "transforms/own_rtl.srt"), rules), lines(t, load(t, "transforms/own_rtl_fixed.srt"), rules))
}

func TestWordsMatchTheStatistics(t *testing.T) {
	t.Parallel()

	s := load(t, "vtt/real/webvttpy_netflix.vtt")
	got, err := validation.Check(s, validation.Rules{MaxWordsPerMinute: 0.001})
	require.NoError(t, err)
	words := 0
	for _, v := range got {
		c := s.Cues()[v.Cue]
		words += int(math.Round(v.Value * (c.End - c.Start).Seconds() / 60))
	}
	assert.Equal(t, stats.Of(s).WordCount, words)
}
