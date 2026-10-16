package model_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// fromTexts builds a subtitle with one cue per text, cue i from i s to i+1 s, as TestSubtitles::fromTexts of
// subtitle-toolbox-php does. An LF in a text splits it into lines.
func fromTexts(texts ...string) *model.Subtitle {
	s := &model.Subtitle{}
	for i, text := range texts {
		start := time.Duration(i) * time.Second
		s.AddCue(model.Cue{Start: start, End: start + time.Second, Lines: strings.Split(text, "\n")})
	}
	return s
}

func readRows[T any](t *testing.T, name string) []T {
	t.Helper()

	var rows []T
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "transform-rows/"+name), &rows))
	require.NotEmpty(t, rows)
	return rows
}

type caseRow struct {
	Name     string         `json:"name"`
	Texts    []string       `json:"texts"`
	Mode     model.CaseMode `json:"mode"`
	Language string         `json:"language"`
	Want     []string       `json:"want"`
}

// Adapted from tests/TextTransformsTest.php and tests/SentenceCaserTest.php of subtitle-toolbox-php.
func TestChangeCase(t *testing.T) {
	t.Parallel()

	const english = "WELL, I'M HERE AND I'LL STAY. I\u2019VE, I'D, I-I, HI, PI, 3I, I'S, I.E."
	rows := slices.Concat(readRows[caseRow](t, "case.json"), []caseRow{
		{Name: "upper keeps tags", Texts: []string{"<i>stop</i>"}, Mode: model.CaseUpper, Want: []string{"<i>STOP</i>"}},
		{
			Name:  "upper keeps tags and entities",
			Texts: []string{`tom &amp; <font color="#ff0000">jerry</font> &lt;3`}, Mode: model.CaseUpper,
			Want: []string{`TOM &amp; <font color="#ff0000">JERRY</font> &lt;3`},
		},
		{Name: "upper keeps &nbsp;", Texts: []string{"yes &nbsp; x"}, Mode: model.CaseUpper, Want: []string{"YES &nbsp; X"}},
		{Name: "upper keeps &#233;", Texts: []string{"yes &#233; x"}, Mode: model.CaseUpper, Want: []string{"YES &#233; X"}},
		{Name: "upper keeps &amp; escaped", Texts: []string{"yes &amp; &nbsp; x"}, Mode: model.CaseUpper, Want: []string{"YES &amp; &nbsp; X"}},
		{Name: "lower of escaped text", Texts: []string{"Q&amp;A; FLUX"}, Mode: model.CaseLower, Want: []string{"q&amp;a; flux"}},
		{Name: "upper of invalid UTF-8 changes ASCII letters only", Texts: []string{"caf\xe9 <i>no\xebl</i>"}, Mode: model.CaseUpper, Want: []string{"CAF\xe9 <i>NO\xebL</i>"}},
		{Name: "lower of invalid UTF-8 changes ASCII letters only", Texts: []string{"CAF\xc9"}, Mode: model.CaseLower, Want: []string{"caf\xc9"}},
		{Name: "lower keeps the English pronoun in lower case", Texts: []string{"I THINK"}, Mode: model.CaseLower, Want: []string{"i think"}},
		{
			Name:  "sentence case",
			Texts: []string{"WHERE ARE YOU GOING? HOME.", `<i>WAIT...</i> <b>WHAT?!</b> "NO." OK.`, "- READ WWW.EXAMPLE.COM.\n- 3.5 KM, THEN STOP!"},
			Mode:  model.CaseSentence,
			Want:  []string{"Where are you going? Home.", `<i>Wait...</i> <b>What?!</b> "No." Ok.`, "- Read www.example.com.\n- 3.5 km, then stop!"},
		},
		{Name: "i.e. and comma", Texts: []string{"it fits, i.e., now or never"}, Mode: model.CaseSentence, Want: []string{"It fits, i.e., now or never"}},
		{Name: "e.g. and comma", Texts: []string{"TAKE ONE, E.G., SOON."}, Mode: model.CaseSentence, Want: []string{"Take one, e.g., soon."}},
		{Name: "e.g. before lower case", Texts: []string{"Fruit, e.g. apples."}, Mode: model.CaseSentence, Want: []string{"Fruit, e.g. apples."}},
		{Name: "vs. before lower case", Texts: []string{"Red vs. blue"}, Mode: model.CaseSentence, Want: []string{"Red vs. blue"}},
		{Name: "etc. before upper case", Texts: []string{"Apples, pears, etc. Then we left."}, Mode: model.CaseSentence, Want: []string{"Apples, pears, etc. Then we left."}},
		{Name: "etc. in upper case text", Texts: []string{"APPLES, ETC. THEN WE LEFT."}, Mode: model.CaseSentence, Want: []string{"Apples, etc. Then we left."}},
		{Name: "comma after a question", Texts: []string{`"WHY?," HE ASKED.`}, Mode: model.CaseSentence, Want: []string{`"Why?," he asked.`}},
		{Name: "tag after i.e.", Texts: []string{"We mean <i>i.e.</i> this one."}, Mode: model.CaseSentence, Want: []string{"We mean <i>i.e.</i> this one."}},
		{Name: "space, tag after i.e.", Texts: []string{"We mean <i>i.e. </i>this one."}, Mode: model.CaseSentence, Want: []string{"We mean <i>i.e. </i>this one."}},
		{Name: "tag before upper case", Texts: []string{"We mean <i>e.g. </i>Then we left."}, Mode: model.CaseSentence, Want: []string{"We mean <i>e.g. </i>Then we left."}},
		{Name: "sentence end in one run capitalizes the next run", Texts: []string{"<i>GO NOW.</i> THIS ONE, <b>OK</b>"}, Mode: model.CaseSentence, Want: []string{"<i>Go now.</i> This one, <b>ok</b>"}},
		{Name: "English pronoun without a language", Texts: []string{english}, Mode: model.CaseSentence, Want: []string{"Well, I'm here and I'll stay. I\u2019ve, I'd, I-I, hi, pi, 3i, i's, i.e."}},
		{
			Name:  "English pronoun is decided on the whole line",
			Texts: []string{"WE &amp; I'LL GO\nI.E., NOW.", "W<b>I</b>NDOW AND I", "<i>I</i> <b>AM</b> H<i>I</i>"},
			Mode:  model.CaseSentence,
			Want:  []string{"We &amp; I'll go\ni.e., now.", "W<b>i</b>ndow and I", "<i>I</i> <b>am</b> h<i>i</i>"},
		},
		{Name: "German keeps a lone lower case i", Texts: []string{"ICH BIN DA, I."}, Mode: model.CaseSentence, Language: "de", Want: []string{"Ich bin da, i."}},
		{Name: "English pronoun in invalid UTF-8", Texts: []string{"CAF\xe9 I THINK"}, Mode: model.CaseSentence, Want: []string{"Caf\xe9 I think"}},
		{Name: "English pronoun before an entity", Texts: []string{"Yes, i &nbsp; x"}, Mode: model.CaseSentence, Want: []string{"Yes, I &nbsp; x"}},
		{Name: "invalid UTF-8 cue ends a sentence", Texts: []string{"\xff END.", "NEXT"}, Mode: model.CaseSentence, Want: []string{"\xff End.", "Next"}},
		{
			Name:  "annotation cues start a sentence in invalid UTF-8",
			Texts: []string{"CAF\xe9 [BELL]", "WAIT \xe9", "NO\xeb JOHN:", "GO \xe9", "(LAUGHS) \xe9", "\u266a LA \xe9 \u266a", "OK"},
			Mode:  model.CaseSentence,
			Want:  []string{"Caf\xe9 [bell]", "Wait \xe9", "no\xeb john:", "Go \xe9", "(Laughs) \xe9", "\u266a La \xe9 \u266a", "Ok"},
		},
		{Name: "empty cue keeps the state", Texts: []string{"HELLO.", "<i></i>", "WORLD"}, Mode: model.CaseSentence, Want: []string{"Hello.", "<i></i>", "World"}},
	})

	for _, tt := range rows {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.Texts...)
			require.NoError(t, s.ChangeCase(tt.Mode, tt.Language))
			assert.Equal(t, tt.Want, texts(s))
		})
	}
}

// Adapted from testGapStartsASentence of tests/TextTransformsTest.php of subtitle-toolbox-php.
func TestSentenceCaseGap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		start time.Duration
		want  string
	}{
		{name: "1.999 s", start: 2999 * time.Millisecond, want: "me"},
		{name: "2 s", start: 3 * time.Second, want: "Me"},
		{name: "5 s", start: 6 * time.Second, want: "Me"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &model.Subtitle{}
			s.AddCues(
				model.Cue{Start: 0, End: time.Second, Lines: []string{"WAIT FOR"}},
				model.Cue{Start: tt.start, End: tt.start + time.Second, Lines: []string{"ME"}},
			)
			require.NoError(t, s.ChangeCase(model.CaseSentence, ""))
			assert.Equal(t, []string{"Wait for", tt.want}, texts(s))
		})
	}
}

func TestChangeCaseRejectsAnUnknownMode(t *testing.T) {
	t.Parallel()

	for _, mode := range []model.CaseMode{"title", "", "Upper"} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			s := fromTexts("Hi")
			err := s.ChangeCase(mode, "")
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var invalid *model.InvalidArgumentError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, `The case mode "`+string(mode)+`" is not upper, lower or sentence.`, invalid.Msg)
			assert.Equal(t, []string{"Hi"}, texts(s))
		})
	}
}

type replaceCall struct {
	Search     string `json:"search"`
	Replace    string `json:"replace"`
	Regex      bool   `json:"regex"`
	IgnoreCase bool   `json:"ignoreCase"`
}

type replaceRow struct {
	Name  string   `json:"name"`
	Texts []string `json:"texts"`
	replaceCall
	Want []string `json:"want"`
}

// Adapted from tests/TextTransformsTest.php of subtitle-toolbox-php.
func TestReplaceText(t *testing.T) {
	t.Parallel()

	type replaceCase struct {
		name  string
		texts []string
		calls []replaceCall
		want  []string
	}
	rows := readRows[replaceRow](t, "replace.json")
	fromJSON := make([]replaceCase, len(rows))
	for i, row := range rows {
		fromJSON[i] = replaceCase{name: row.Name, texts: row.Texts, calls: []replaceCall{row.replaceCall}, want: row.Want}
	}
	tests := slices.Concat(fromJSON, []replaceCase{
		{
			name:  "matches the text between tags",
			texts: []string{"<i>Colour</i> me surprised", "Tom &amp; Jerry"},
			calls: []replaceCall{{Search: "Colour", Replace: "Color"}, {Search: "&", Replace: "and"}},
			want:  []string{"<i>Color</i> me surprised", "Tom and Jerry"},
		},
		{
			name:  "tags never match",
			texts: []string{`<font color="#ff0000">red</font> amp`},
			calls: []replaceCall{{Search: "ff", Replace: "XX"}, {Search: "amp", Replace: "lamp"}},
			want:  []string{`<font color="#ff0000">red</font> lamp`},
		},
		{
			name:  "escapes the replacement",
			texts: []string{"I love it", "<b>x</b>"},
			calls: []replaceCall{{Search: "love", Replace: "<3 & more"}, {Search: "x", Replace: "<b>"}},
			want:  []string{"I &lt;3 &amp; more it", "<b>&lt;b&gt;</b>"},
		},
		{
			name:  "keeps raw WebVTT characters of the unchanged parts",
			texts: []string{">> TOM & JERRY", ">> A &amp; B"},
			calls: []replaceCall{{Search: "TOM", Replace: "Tom &lt;"}, {Search: "A", Replace: "a"}},
			want:  []string{">> Tom &amp;lt; & JERRY", ">> a &amp; B"},
		},
		{
			name:  "regex",
			texts: []string{"Wait....."},
			calls: []replaceCall{{Search: `\.{4,}`, Replace: "...", Regex: true}},
			want:  []string{"Wait..."},
		},
		{
			name:  "regex ignoring case with a group",
			texts: []string{"<i>colour</i> and COLOUR"},
			calls: []replaceCall{{Search: "col(ou)r", Replace: "col$1r!", Regex: true, IgnoreCase: true}},
			want:  []string{"<i>colour!</i> and colOUr!"},
		},
		{
			name:  "regex substitutions",
			texts: []string{"ab cd"},
			calls: []replaceCall{{Search: `(\w)(?<w>\w)`, Replace: "${w}$1$$", Regex: true}},
			want:  []string{"ba$ dc$"},
		},
		{
			name:  "regex groups numbered in pattern order",
			texts: []string{"ab"},
			calls: []replaceCall{{Search: `(?<n>a)(b)`, Replace: "[$1|$2|${n}]", Regex: true}},
			want:  []string{"[a|b|a]"},
		},
		{
			name:  "regex lookbehind",
			texts: []string{"a1 b1"},
			calls: []replaceCall{{Search: `(?<=b)1`, Replace: "2", Regex: true}},
			want:  []string{"a1 b2"},
		},
		{
			name:  "regex case-sensitive by default",
			texts: []string{"Colour COLOUR"},
			calls: []replaceCall{{Search: "COLOUR", Replace: "x", Regex: true}},
			want:  []string{"Colour x"},
		},
		{
			name:  "regex keeps a run of invalid UTF-8",
			texts: []string{"<i>x</i> caf\xe9 x"},
			calls: []replaceCall{{Search: "x", Replace: "y", Regex: true}},
			want:  []string{"<i>y</i> caf\xe9 x"},
		},
		{
			name:  "plain ignoring case keeps $ and backslash literal",
			texts: []string{"Price: $1"},
			calls: []replaceCall{{Search: "price", Replace: `$1 \1`, IgnoreCase: true}},
			want:  []string{`$1 \1: $1`},
		},
		{
			name:  "plain ignoring case quotes the search",
			texts: []string{"a.b axb"},
			calls: []replaceCall{{Search: "A.B", Replace: "z", IgnoreCase: true}},
			want:  []string{"z axb"},
		},
		{
			name:  "plain ignoring case in invalid UTF-8 matches ASCII letters only",
			texts: []string{"caf\xe9 CAFE Cafe", "\xc9t\xe9 \xe9T\xe9"},
			calls: []replaceCall{{Search: "cafe", Replace: "x", IgnoreCase: true}, {Search: "\xe9t\xe9", Replace: "y", IgnoreCase: true}},
			want:  []string{"caf\xe9 x x", "\xc9t\xe9 y"},
		},
		{
			name:  "plain case-sensitive replaces bytes",
			texts: []string{"caf\xe9 au caf\xe9"},
			calls: []replaceCall{{Search: "\xe9", Replace: "e"}},
			want:  []string{"cafe au cafe"},
		},
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.texts...)
			for _, c := range tt.calls {
				require.NoError(t, s.ReplaceText(c.Search, c.Replace, model.ReplaceOptions{Regex: c.Regex, IgnoreCase: c.IgnoreCase}))
			}
			assert.Equal(t, tt.want, texts(s))
		})
	}
}

func TestReplaceTextErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		search, replace string
		opts            model.ReplaceOptions
		msg             string
	}{
		{name: "empty search", opts: model.ReplaceOptions{}, msg: "The search text must not be empty."},
		{name: "empty regex", opts: model.ReplaceOptions{Regex: true}, msg: "The search text must not be empty."},
		{
			name: "invalid regex", search: "(", opts: model.ReplaceOptions{Regex: true},
			msg: "The regular expression \"(\" is not valid: error parsing regexp: missing closing ) in `(`.",
		},
		{
			name: "group number too large in the replacement", search: "(a)", replace: "$99999999999", opts: model.ReplaceOptions{Regex: true},
			msg: "The replacement \"$99999999999\" is not valid: error parsing regexp: capture group number out of range in `$99999999999`.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts("abc")
			err := s.ReplaceText(tt.search, tt.replace, tt.opts)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var invalid *model.InvalidArgumentError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, tt.msg, invalid.Msg)
			assert.Equal(t, []string{"abc"}, texts(s))
		})
	}
}

func TestReplaceTextTimeoutChangesNothing(t *testing.T) {
	t.Parallel()

	letters := strings.Repeat("a", 40)
	tests := []struct {
		name    string
		texts   []string
		pattern string
	}{
		{name: "timeout on the first match of a run", texts: []string{"x marks the spot", letters + "!"}, pattern: `x|(a+)+$`},
		{name: "timeout after a match in the same run", texts: []string{"keep", "x " + letters}, pattern: `x|(a+)+b`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.texts...)
			err := s.ReplaceText(tt.pattern, "y", model.ReplaceOptions{Regex: true})
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var invalid *model.InvalidArgumentError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, `The regular expression "`+tt.pattern+`" took longer than 1 s on cue 2.`, invalid.Msg)
			assert.Equal(t, tt.texts, texts(s))
		})
	}
}

// Adapted from testRemovesCuesThatBecomeEmptyAndKeepsComments of tests/TextTransformsTest.php of subtitle-toolbox-php.
func TestReplaceTextRemovesEmptiedCues(t *testing.T) {
	t.Parallel()

	s := fromTexts("first", "<i>[MUSIC]</i>", "[DOOR]\n[MUSIC]", "last")
	s.AddCue(model.Cue{Start: 10 * time.Second, End: 11 * time.Second})
	require.NoError(t, s.AddComment("before music", 1))
	require.NoError(t, s.AddComment("before door", 2))
	require.NoError(t, s.AddComment("before last", 3))

	require.NoError(t, s.ReplaceText(`\[\w+\]`, "", model.ReplaceOptions{Regex: true}))

	assert.Equal(t, []string{"first", "last", ""}, texts(s))
	assert.Equal(t, []model.Comment{
		{Text: "before music", BeforeCue: 1},
		{Text: "before door", BeforeCue: 1},
		{Text: "before last", BeforeCue: 1},
	}, s.Comments())
}

// Adapted from testStripFormatting and testStripFormattingKeepsWordTimestamps of tests/TextTransformsTest.php of
// subtitle-toolbox-php.
func TestStripFormatting(t *testing.T) {
	t.Parallel()

	const timed = "<b>One</b> <00:00:01.500>two <00:00:02.000><i>three</i>"
	tests := []struct {
		name string
		line string
		opts model.StripOptions
		want []string
	}{
		{name: "every tag", line: `<b>Run</b>, <font color="#ff0000">now</font>!`, want: []string{"Run, now!"}},
		{name: "voice tag and entity", line: "<b><i>Run</i></b> &amp; <v Fred>hide", want: []string{"Run &amp; hide"}},
		{name: "keep list", line: "<b><i>Run</i></b> &amp; <v Fred>hide", opts: model.StripOptions{KeepTags: []string{"i"}}, want: []string{"<i>Run</i> &amp; hide"}},
		{name: "keep list in upper case", line: "<b><I>Run</I></b>", opts: model.StripOptions{KeepTags: []string{"I"}}, want: []string{"<I>Run</I>"}},
		{name: "keeps word timestamps", line: timed, want: []string{"One <00:00:01.500>two <00:00:02.000>three"}},
		{
			name: "removes word timestamps", line: timed,
			opts: model.StripOptions{KeepTags: []string{"i"}, RemoveWordTimestamps: true}, want: []string{"One two <i>three</i>"},
		},
		{name: "ASS override block is no tag", line: `{\an8}<i>Hi</i>`, want: []string{`{\an8}Hi`}},
		{name: "a cue without text stays", line: "<i></i>", want: []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.line)
			s.StripFormatting(tt.opts)
			assert.Equal(t, tt.want, texts(s))
		})
	}
}

// Adapted from testMapTextGetsDecodedTextRunsAndTheCue of tests/TextTransformsTest.php of subtitle-toolbox-php.
func TestMapText(t *testing.T) {
	t.Parallel()

	type call struct {
		text  string
		start time.Duration
	}
	s := fromTexts("<i>a &amp; b</i> c", "&lt;d&gt;")
	var calls []call
	s.MapText(func(text string, c model.Cue) string {
		calls = append(calls, call{text, c.Start})
		return "[" + text + "]"
	})

	assert.Equal(t, []call{{"a & b", 0}, {" c", 0}, {"<d>", time.Second}}, calls)
	assert.Equal(t, []string{"<i>[a &amp; b]</i>[ c]", "[&lt;d&gt;]"}, texts(s))
}

func TestMapTextKeepsEntities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, line string
		fn         func(string, model.Cue) string
		want       string
	}{
		{name: "unchanged runs keep their bytes", line: "a&nbsp;b <i>&#39;c&#39;</i>", fn: func(s string, _ model.Cue) string { return s }, want: "a&nbsp;b <i>&#39;c&#39;</i>"},
		{name: "changed run keeps &nbsp;", line: "x &nbsp; x", fn: func(s string, _ model.Cue) string { return strings.ReplaceAll(s, "x", "y") }, want: "y &nbsp; y"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.line)
			s.MapText(tt.fn)
			assert.Equal(t, []string{tt.want}, texts(s))
		})
	}
}

// Adapted from testMapLinesGetsFullLines of tests/TextTransformsTest.php of subtitle-toolbox-php.
func TestMapLines(t *testing.T) {
	t.Parallel()

	s := fromTexts("one\n<i>two</i>", "three")
	var starts []time.Duration
	s.MapLines(func(line string, c model.Cue) string {
		starts = append(starts, c.Start)
		return `<font color="#ffff00">` + line + "</font>"
	})

	assert.Equal(t, []string{`<font color="#ffff00">one</font>` + "\n" + `<font color="#ffff00"><i>two</i></font>`, `<font color="#ffff00">three</font>`}, texts(s))
	assert.Equal(t, []time.Duration{0, 0, time.Second}, starts)
}

type linesRow struct {
	Name  string   `json:"name"`
	Texts []string `json:"texts"`
	Want  []string `json:"want"`
}

func TestFixRightToLeft(t *testing.T) {
	t.Parallel()

	for _, tt := range readRows[linesRow](t, "rtl.json") {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.Texts...)
			s.FixRightToLeft()
			assert.Equal(t, tt.Want, texts(s))
			s.FixRightToLeft()
			assert.Equal(t, tt.Want, texts(s), "idempotent")
		})
	}
}

// Adapted from bidiControlProvider of tests/TextTransformsTest.php of subtitle-toolbox-php.
func TestRemoveBidiControls(t *testing.T) {
	t.Parallel()

	rows := slices.Concat(readRows[linesRow](t, "bidi.json"), []linesRow{
		{Name: "override", Texts: []string{"Hello \u202edlrow"}, Want: []string{"Hello dlrow"}},
		{Name: "isolates", Texts: []string{"\u2066a\u2069 \u2067b\u2069 \u2068c\u2069\nd\u202ae\u202d"}, Want: []string{"a b c\nde"}},
		{Name: "no-break space", Texts: []string{"a\u00a0b"}, Want: []string{"a\u00a0b"}},
		{Name: "invalid UTF-8", Texts: []string{"\xff\u200fa"}, Want: []string{"\xffa"}},
		{Name: "a cue of controls only goes", Texts: []string{"a", "<i>\u200f</i>", "b"}, Want: []string{"a", "b"}},
	})
	for _, tt := range rows {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.Texts...)
			s.RemoveBidiControls()
			assert.Equal(t, tt.Want, texts(s))
		})
	}
}

func TestEditLines(t *testing.T) {
	t.Parallel()

	t.Run("removes the cues that had text and have none", func(t *testing.T) {
		t.Parallel()

		s := fromTexts("keep", "drop", "<i></i>", "drop too", "last")
		require.NoError(t, s.AddComment("before drop", 1))
		require.NoError(t, s.AddComment("before drop too", 3))
		require.NoError(t, s.AddComment("at the end", 5))

		removed := s.EditLines(func(_ int, c model.Cue) ([]string, bool) {
			if strings.HasPrefix(c.Text(), "drop") {
				return []string{"<b> </b>"}, true
			}
			return []string{"  " + c.Text() + "  ", ""}, true
		}, nil)

		assert.Equal(t, []int{1, 3}, removed)
		assert.Equal(t, []string{"keep", "<i></i>", "last"}, texts(s))
		assert.Equal(t, []model.Comment{
			{Text: "before drop", BeforeCue: 1},
			{Text: "before drop too", BeforeCue: 2},
			{Text: "at the end", BeforeCue: 3},
		}, s.Comments())
	})

	t.Run("false keeps the cue", func(t *testing.T) {
		t.Parallel()

		s := fromTexts("a", "b")
		var indexes []int
		removed := s.EditLines(func(i int, c model.Cue) ([]string, bool) {
			indexes = append(indexes, i)
			c.Lines[0] = "changed copy"
			return nil, false
		}, nil)

		assert.Empty(t, removed)
		assert.Equal(t, []int{0, 1}, indexes)
		assert.Equal(t, []string{"a", "b"}, texts(s))
	})

	t.Run("hasText decides", func(t *testing.T) {
		t.Parallel()

		s := fromTexts("[MUSIC]", "words")
		bracketsOnly := func(lines []string) bool {
			return regexp.MustCompile(`[^\[\]A-Z ]`).MatchString(strings.Join(lines, ""))
		}
		removed := s.EditLines(func(_ int, c model.Cue) ([]string, bool) {
			return []string{strings.ToUpper(c.Text())}, true
		}, bracketsOnly)

		assert.Equal(t, []int{1}, removed)
		assert.Equal(t, []string{"[MUSIC]"}, texts(s))
	})
}

func TestDialogueDashStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		style      model.DialogueDashStyle
		text, name string
	}{
		{model.DialogueDashHyphenSpace, "- ", "DialogueDashHyphenSpace"},
		{model.DialogueDashHyphen, "-", "DialogueDashHyphen"},
		{model.DialogueDashUnicodeHyphen, "\u2010", "DialogueDashUnicodeHyphen"},
		{model.DialogueDashUnicodeHyphenSpace, "\u2010 ", "DialogueDashUnicodeHyphenSpace"},
		{model.DialogueDashEnDash, "\u2013", "DialogueDashEnDash"},
		{model.DialogueDashEnDashSpace, "\u2013 ", "DialogueDashEnDashSpace"},
		{model.DialogueDashEmDash, "\u2014", "DialogueDashEmDash"},
		{model.DialogueDashEmDashSpace, "\u2014 ", "DialogueDashEmDashSpace"},
		{model.DialogueDashStyle(8), "", "DialogueDashStyle(8)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.text, tt.style.Text())
			assert.Equal(t, tt.name, tt.style.String())
		})
	}
	var zero model.DialogueDashStyle
	assert.Equal(t, model.DialogueDashHyphenSpace, zero)
}

// readTransformFixture reads a fixture of testdata/transforms with the SubRip or the WebVTT reader.
func readTransformFixture(tb testing.TB, name string) *model.Subtitle {
	tb.Helper()

	f := bytes.NewReader(testfiles.Read(tb, "transforms/"+name))
	var s *model.Subtitle
	var warnings []model.ParseWarning
	var err error
	if strings.HasSuffix(name, ".vtt") {
		s, warnings, err = webvtt.Read(f, model.CommonReadOptions{}, webvtt.ReadOptions{})
	} else {
		s, warnings, err = srt.Read(f, model.CommonReadOptions{}, srt.ReadOptions{})
	}
	require.NoError(tb, err)
	require.Empty(tb, warnings)
	return s
}

func TestTransformFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		in, want   string
		edit       func(*model.Subtitle) error
		idempotent bool
	}{
		{
			name: "CEA-608 captions in sentence case", in: "own_cea608_caps.vtt", want: "own_cea608_caps_sentence.vtt",
			edit: func(s *model.Subtitle) error { return s.ChangeCase(model.CaseSentence, "") },
		},
		{
			name: "run-on captions continue sentences across cues", in: "own_cea608_run_on.vtt", want: "own_cea608_run_on_sentence.vtt",
			edit: func(s *model.Subtitle) error { return s.ChangeCase(model.CaseSentence, "") },
		},
		{
			name: "sound cues and speaker labels start sentences", in: "own_cea608_sound_cues.vtt", want: "own_cea608_sound_cues_sentence.vtt",
			edit: func(s *model.Subtitle) error { return s.ChangeCase(model.CaseSentence, "") },
		},
		{
			name: "CEA-608 captions cleaned up", in: "own_cea608_caps.vtt", want: "own_cea608_caps_cleaned.vtt",
			edit: func(s *model.Subtitle) error {
				err := errors.Join(
					s.ReplaceText(`\[[^\]]*\]`, "", model.ReplaceOptions{Regex: true}),
					s.ReplaceText(`\.{4,}`, "...", model.ReplaceOptions{Regex: true}),
				)
				s.StripFormatting(model.StripOptions{})
				return err
			},
		},
		{
			name: "CEA-608 captions without tags", in: "own_cea608_caps.vtt", want: "own_cea608_caps_stripped.vtt",
			edit: func(s *model.Subtitle) error { s.StripFormatting(model.StripOptions{}); return nil },
		},
		{
			name: "multilingual upper case", in: "own_multilingual_caps.srt", want: "own_multilingual_caps_upper.srt",
			edit: func(s *model.Subtitle) error { return s.ChangeCase(model.CaseUpper, "") },
		},
		{
			name: "multilingual lower case with Turkish rules", in: "own_multilingual_caps.srt", want: "own_multilingual_caps_lower_tr.srt",
			edit: func(s *model.Subtitle) error { return s.ChangeCase(model.CaseLower, "tr") },
		},
		{
			name: "multilingual replace ignoring case", in: "own_multilingual_caps.srt", want: "own_multilingual_caps_replaced.srt",
			edit: func(s *model.Subtitle) error {
				return s.ReplaceText("uhr", "Uhr", model.ReplaceOptions{IgnoreCase: true})
			},
		},
		{
			name: "right-to-left lines", in: "own_rtl.srt", want: "own_rtl_fixed.srt",
			edit: func(s *model.Subtitle) error { s.FixRightToLeft(); return nil }, idempotent: true,
		},
		{
			name: "right-to-left lines that start with their punctuation", in: "own_rtl_reversed.srt", want: "own_rtl_reversed_fixed.srt",
			edit: func(s *model.Subtitle) error { s.FixRightToLeft(); return nil }, idempotent: true,
		},
		{
			name: "removing the bidi controls undoes the fix", in: "own_rtl_fixed.srt", want: "own_rtl.srt",
			edit: func(s *model.Subtitle) error { s.RemoveBidiControls(); return nil },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readTransformFixture(t, tt.in)
			require.NoError(t, tt.edit(s))
			want := readTransformFixture(t, tt.want)
			assert.Equal(t, timesAndLines(want), timesAndLines(s))
			assert.Equal(t, want.Comments(), s.Comments())

			if tt.idempotent {
				require.NoError(t, tt.edit(s))
				assert.Equal(t, timesAndLines(want), timesAndLines(s), "idempotent")
			}
		})
	}
}

// Adapted from testRealMultilingualFileInLowerCase and testTurkishRulesApplyToEveryCue of
// tests/TextTransformsTest.php of subtitle-toolbox-php.
func TestMultilingualFixture(t *testing.T) {
	t.Parallel()

	for _, tt := range readRows[caseRow](t, "multilingual.json") {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			s := readTransformFixture(t, "own_multilingual_caps.srt")
			require.NoError(t, s.ChangeCase(tt.Mode, tt.Language))
			assert.Equal(t, tt.Want, texts(s))
		})
	}
}
