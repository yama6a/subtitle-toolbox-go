package fixing_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/fixing"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// only turns every rule off but r.
func only(r fixing.Rule) fixing.Options {
	var opts fixing.Options
	for _, other := range fixing.Rules() {
		if other != r && other.Default() {
			opts.Disable = append(opts.Disable, other)
		}
	}
	if !r.Default() {
		opts.Enable = []fixing.Rule{r}
	}
	return opts
}

func allOff() fixing.Options {
	opts := only(fixing.RuleAdCues)
	opts.Enable = nil
	return opts
}

func withLanguage(opts fixing.Options, language string) fixing.Options {
	opts.Language = language
	return opts
}

// fromTexts gives one cue per text, the cue i from 2i s to 2i+1 s.
func fromTexts(texts ...string) *model.Subtitle {
	var s model.Subtitle
	for i, text := range texts {
		c := model.Cue{Start: time.Duration(2*i) * time.Second, End: time.Duration(2*i+1) * time.Second}
		c.SetText(text)
		s.AddCue(c)
	}
	return &s
}

func fixLines(t *testing.T, lines []string, opts fixing.Options) ([]string, []fixing.Fix) {
	t.Helper()

	var s model.Subtitle
	s.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: lines})
	report, err := fixing.Apply(&s, opts)
	require.NoError(t, err)
	c, _ := s.Cue(0)
	return c.Lines, report.Fixes
}

func texts(s *model.Subtitle) []string {
	out := []string{}
	for _, c := range s.All() {
		out = append(out, c.Text())
	}
	return out
}

func rules(fixes []fixing.Fix) []fixing.Rule {
	out := make([]fixing.Rule, 0, len(fixes))
	for _, f := range fixes {
		out = append(out, f.Rule)
	}
	return out
}

func read(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	return s
}

type cueLines struct {
	Start, End time.Duration
	Lines      []string
}

func cuesOf(s *model.Subtitle) []cueLines {
	var out []cueLines
	for _, c := range s.All() {
		out = append(out, cueLines{c.Start, c.End, c.Lines})
	}
	return out
}

type ruleRow struct {
	Name     string      `json:"name"`
	Rule     fixing.Rule `json:"rule"`
	Language string      `json:"language"`
	Before   []string    `json:"before"`
	After    []string    `json:"after"`
}

func TestRuleFixesTheTextBetweenTags(t *testing.T) {
	t.Parallel()

	rows := []ruleRow{
		{"double spaces across a tag", fixing.RuleDoubleSpaces, "en", []string{"Hi <i> there</i>"}, []string{"Hi <i>there</i>"}},
		{"space before a question mark", fixing.RuleSpaceBeforePunctuation, "en", []string{"Really ?"}, []string{"Really?"}},
		{"space before a comma and a period", fixing.RuleSpaceBeforePunctuation, "de", []string{"Ja , gut ."}, []string{"Ja, gut."}},
		{"French keeps the space before ?", fixing.RuleSpaceBeforePunctuation, "fr", []string{"Quoi ? Non , merci !"}, []string{"Quoi ? Non, merci !"}},
		{"space before an ellipsis stays", fixing.RuleSpaceBeforePunctuation, "en", []string{"Wait ... now"}, []string{"Wait ... now"}},
		{"space before a smiley stays", fixing.RuleSpaceBeforePunctuation, "en", []string{"Great :)"}, []string{"Great :)"}},
		{"missing space after a period", fixing.RuleMissingSpaceAfterPunctuation, "en", []string{"Stop.Now"}, []string{"Stop. Now"}},
		{"numbers, URLs and abbreviations", fixing.RuleMissingSpaceAfterPunctuation, "en", []string{"1.5 www.example.com e.g. U.S.Army ASP.NET"}, []string{"1.5 www.example.com e.g. U.S.Army ASP.NET"}},
		{"unclosed tag", fixing.RuleUnbalancedTags, "en", []string{"<i>Hello"}, []string{"<i>Hello</i>"}},
		{"tag over two lines", fixing.RuleUnbalancedTags, "en", []string{"<i>Hello", "world</i>"}, []string{"<i>Hello", "world</i>"}},
		{"unclosed tags close in reverse", fixing.RuleUnbalancedTags, "en", []string{"<b><font color=\"#ff0000\">Hi", "you"}, []string{"<b><font color=\"#ff0000\">Hi", "you</font></b>"}},
		{"stray closing tag", fixing.RuleUnbalancedTags, "en", []string{"Hello</i> there"}, []string{"Hello there"}},
		{"speaker tag needs no closing tag", fixing.RuleUnbalancedTags, "en", []string{"<v Anna>Hello"}, []string{"<v Anna>Hello"}},
		{"closing tag of a WebVTT class tag", fixing.RuleUnbalancedTags, "en", []string{"<b.loud>Hello</b> there"}, []string{"<b.loud>Hello</b> there"}},
		{"empty tag", fixing.RuleEmptyTags, "en", []string{"Hi <i></i>there"}, []string{"Hi there"}},
		{"nested empty tags", fixing.RuleEmptyTags, "en", []string{"Hi <b><i> </i></b>there"}, []string{"Hi there"}},
		{"dialogue dashes", fixing.RuleDialogueDashes, "en", []string{"-Hi.", "-Hello."}, []string{"- Hi.", "- Hello."}},
		{"negative number", fixing.RuleDialogueDashes, "en", []string{"-5 degrees."}, []string{"-5 degrees."}},
		{"double dash", fixing.RuleDialogueDashes, "en", []string{"--and then"}, []string{"--and then"}},
		{"spaced dots", fixing.RuleEllipsis, "en", []string{"Well. . . no."}, []string{"Well... no."}},
		{"four dots", fixing.RuleEllipsis, "en", []string{"Well...."}, []string{"Well..."}},
		{"lt's, l'm and l'll", fixing.RuleOCRLowercaseL, "en", []string{"lt's late. l'm here, l'll go."}, []string{"It's late. I'm here, I'll go."}},
		{"standalone l", fixing.RuleOCRLowercaseL, "en", []string{"lf l can, l will.", "-l see."}, []string{"If I can, I will.", "-I see."}},
		{"units and names stay", fixing.RuleOCRLowercaseL, "en", []string{"5 lbs, 2 l, Al and llamas."}, []string{"5 lbs, 2 l, Al and llamas."}},
		{"l in upper case words", fixing.RuleOCRLowercaseL, "it", []string{"NON LO SO. lL MlO."}, []string{"NON LO SO. IL MIO."}},
		{"other languages", fixing.RuleOCRLowercaseL, "it", []string{"lo sono lt"}, []string{"lo sono lt"}},
		{"pipe at a word start", fixing.RuleOCRPipe, "en", []string{"|t was"}, []string{"It was"}},
		{"pipe after lower case", fixing.RuleOCRPipe, "de", []string{"Wi|| he|fen"}, []string{"Will helfen"}},
		{"standalone pipe in English", fixing.RuleOCRPipe, "en", []string{"Yes, | know. |'m here."}, []string{"Yes, I know. I'm here."}},
		{"pipe and apostrophe in French", fixing.RuleOCRPipe, "fr", []string{"|'homme | 2"}, []string{"l'homme | 2"}},
		{"zero in an upper case word", fixing.RuleOCRZeroInWords, "en", []string{"D0N'T"}, []string{"DON'T"}},
		{"zero in lower case words", fixing.RuleOCRZeroInWords, "en", []string{"0nly the n0rth. 0nce"}, []string{"Only the north. Once"}},
		{"numbers stay", fixing.RuleOCRZeroInWords, "en", []string{"007, 2.0, 10am, 0s and 3D0"}, []string{"007, 2.0, 10am, 0s and 3D0"}},
		{"entities stay escaped", fixing.RuleOCRLowercaseL, "en", []string{"&lt;lt&gt; &amp; l"}, []string{"&lt;It&gt; &amp; I"}},
		{"dialogue on one line", fixing.RuleDialogueOnOneLine, "en", []string{"- Hi. - Hello."}, []string{"- Hi.", "- Hello."}},
		{"dialogue on one line in italics", fixing.RuleDialogueOnOneLine, "en", []string{"<i>- Hi. - Hello.</i>"}, []string{"<i>- Hi.</i>", "<i>- Hello.</i>"}},
		{"dialogue without the first dash", fixing.RuleDialogueOnOneLine, "en", []string{"Hi. - Hello."}, []string{"- Hi.", "- Hello."}},
		{"dash after a question in a tag", fixing.RuleDialogueOnOneLine, "en", []string{"<font color=\"#ffff00\">Why?</font> -Because."}, []string{"<font color=\"#ffff00\">- Why?</font>", "-Because."}},
		{"dialogue over two lines", fixing.RuleDialogueOnOneLine, "en", []string{"Hi. - Hello,", "how are you?"}, []string{"- Hi.", "- Hello, how are you?"}},
		{"dash inside a sentence", fixing.RuleDialogueOnOneLine, "en", []string{"A well-known - and loved - song."}, []string{"A well-known - and loved - song."}},
		{"dialogue already on two lines", fixing.RuleDialogueOnOneLine, "en", []string{"- Hi.", "- Hello."}, []string{"- Hi.", "- Hello."}},
		{"three speakers stay", fixing.RuleDialogueOnOneLine, "en", []string{"- Hi. - Hello. - Hey."}, []string{"- Hi. - Hello. - Hey."}},
		{"negative number after a sentence", fixing.RuleDialogueOnOneLine, "en", []string{"It's cold. -5 degrees."}, []string{"It's cold. -5 degrees."}},
		{"lone i", fixing.RuleLoneLowercaseI, "en", []string{"i think i can."}, []string{"I think I can."}},
		{"lone i in a tag", fixing.RuleLoneLowercaseI, "en", []string{"<i>i</i> know"}, []string{"<i>I</i> know"}},
		{"i in words and abbreviations", fixing.RuleLoneLowercaseI, "en", []string{"see i.e. here, www.i.com, iPhone, w<b>i</b>th"}, []string{"see i.e. here, www.i.com, iPhone, w<b>i</b>th"}},
		{"lone i in German", fixing.RuleLoneLowercaseI, "de", []string{"i think"}, []string{"i think"}},
		{"lone i without a language", fixing.RuleLoneLowercaseI, "", []string{"i think"}, []string{"i think"}},
		{"first cue starts a sentence", fixing.RuleSentenceStartCase, "en", []string{"hello."}, []string{"Hello."}},
		{"line after a sentence end", fixing.RuleSentenceStartCase, "en", []string{"- Hello.", "- <i>\"where</i> are you?"}, []string{"- Hello.", "- <i>\"Where</i> are you?"}},
		{"line after a comma", fixing.RuleSentenceStartCase, "en", []string{"So I said,", "no way."}, []string{"So I said,", "no way."}},
		{"word with an inner capital", fixing.RuleSentenceStartCase, "en", []string{"iPhone is here."}, []string{"iPhone is here."}},
		{"signs that are no music", fixing.RuleMusicNotes, "en", []string{"Room #5, *sigh*", "C# code", "#1 fan"}, []string{"Room #5, *sigh*", "C# code", "#1 fan"}},
		{"double apostrophes", fixing.RuleDoubleApostrophes, "en", []string{"''Hi,'' she said."}, []string{"\"Hi,\" she said."}},
		{"double apostrophes in a tag", fixing.RuleDoubleApostrophes, "en", []string{"<i>''Run!''</i>"}, []string{"<i>\"Run!\"</i>"}},
		{"double apostrophes after It's", fixing.RuleDoubleApostrophes, "en", []string{"It's ''the'' one"}, []string{"It's \"the\" one"}},
		{"single apostrophes", fixing.RuleDoubleApostrophes, "en", []string{"rock 'n' roll 'til dawn"}, []string{"rock 'n' roll 'til dawn"}},
		{"apostrophes in a tag attribute", fixing.RuleDoubleApostrophes, "en", []string{"<font face=''>Hi</font>"}, []string{"<font face=''>Hi</font>"}},
		{"period after a question", fixing.RuleUnneededPeriods, "en", []string{"Really?."}, []string{"Really?"}},
		{"period after a bang in a tag", fixing.RuleUnneededPeriods, "en", []string{"<i>Stop!.</i>"}, []string{"<i>Stop!</i>"}},
		{"period inside a line", fixing.RuleUnneededPeriods, "en", []string{"What?. Why?"}, []string{"What? Why?"}},
		{"lone i in invalid UTF-8 stays", fixing.RuleLoneLowercaseI, "en", []string{"caf\xe9 i think"}, []string{"caf\xe9 i think"}},
		{"site address", fixing.RuleAdCues, "en", []string{"Downloaded from www.OpenSubtitles.org"}, nil},
		{"site name with a top level domain", fixing.RuleAdCues, "en", []string{"Subtitles: YTS.MX"}, nil},
		{"ad line and dialogue line", fixing.RuleAdCues, "en", []string{"Ripped by YIFY", "- Hi there."}, nil},
		{"Slovenian word for subtitles", fixing.RuleAdCues, "en", []string{"Prevodi: podnapisi"}, []string{"Prevodi: podnapisi"}},
		{"English phrase", fixing.RuleAdCues, "en", []string{"I read open subtitles."}, []string{"I read open subtitles."}},
		{"l before 2 upper case letters", fixing.RuleOCRLowercaseL, "it", []string{"lT lS"}, []string{"IT IS"}},
		{"pipe before invalid UTF-8", fixing.RuleOCRPipe, "en", []string{"a| caf\xe9"}, []string{"al caf\xe9"}},
		{"pipe after invalid UTF-8", fixing.RuleOCRPipe, "en", []string{"caf\xe9 |"}, []string{"caf\xe9 |"}},
		{"spaces in a tag pair", fixing.RuleDoubleSpaces, "en", []string{"a <i>  </i> b"}, []string{"a <i></i>b"}},
		{"zero after a dash and an inverted question mark", fixing.RuleOCRZeroInWords, "en", []string{"- 0ver. \u00bf0tra?"}, []string{"- Over. \u00bfOtra?"}},
		{"zero word in the second run", fixing.RuleOCRZeroInWords, "en", []string{"<i>x</i> 0ver"}, []string{"<i>x</i> over"}},
		{"stray and open tags", fixing.RuleUnbalancedTags, "en", []string{"</b>a</i>", "<i>b"}, []string{"a", "<i>b</i>"}},
		{"dialogue in 2 tags", fixing.RuleDialogueOnOneLine, "en", []string{"<i>Hi.</i> <b>- Hello.</b>"}, []string{"<i>- Hi.</i>", "<b>- Hello.</b>"}},
		{"music sign in a tag pair", fixing.RuleMusicNotes, "en", []string{"<i>#</i>"}, []string{"<i>\u266a</i>"}},
		{"pipe after a line feed inside a line", fixing.RuleOCRPipe, "fr", []string{"Oui\n| dit"}, []string{"Oui\n| dit"}},
		{"pipe before a letter after a line feed", fixing.RuleOCRPipe, "it", []string{"si\n|o"}, []string{"si\nIo"}},
		{"space after a line feed inside a line", fixing.RuleDoubleSpaces, "en", []string{"a \n<i> b</i>"}, []string{"a \n<i> b</i>"}},
		{"5 spaced dots", fixing.RuleEllipsis, "en", []string{". . . . ."}, []string{"..."}},
	}
	var fromFile []ruleRow
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "fixing/rule-rows.json"), &fromFile))
	require.NotEmpty(t, fromFile)
	for _, tt := range append(rows, fromFile...) {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			lines, fixes := fixLines(t, tt.Before, withLanguage(only(tt.Rule), tt.Language))
			assert.Equal(t, tt.After, lines)
			want := []fixing.Rule{tt.Rule}
			if strings.Join(tt.Before, "\n") == strings.Join(tt.After, "\n") {
				want = []fixing.Rule{}
			}
			assert.Equal(t, want, rules(fixes))
		})
	}
}

func TestFixesTheFileAsTheGoldenFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input, language, golden string
	}{
		{"web-errors.srt", "en", "web-errors.fixed.srt"},
		{"ocr-en.ocr.srt", "en", "ocr-en.fixed.srt"},
		{"ocr-de.ocr.srt", "de", "ocr-de.fixed.srt"},
		{"ocr-fr.ocr.srt", "fr", "ocr-fr.fixed.srt"},
		{"ocr-es.ocr.srt", "es", "ocr-es.fixed.srt"},
		{"text_1080p.ocr.srt", "en", "text_1080p.fixed.srt"},
		{"rtl-dialogue.srt", "he", "rtl-dialogue.fixed.srt"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			s := read(t, "fixing/"+tt.input)
			opts := fixing.Options{Language: tt.language}
			report, err := fixing.Apply(s, opts)
			require.NoError(t, err)
			golden := read(t, "fixing/"+tt.golden)
			assert.Equal(t, cuesOf(golden), cuesOf(s))
			assert.NotEmpty(t, report.Fixes)

			again, err := fixing.Apply(golden, opts)
			require.NoError(t, err)
			assert.Empty(t, again.Fixes)
		})
	}
}

func TestOptionalRulesFixTheFileAsTheGoldenFile(t *testing.T) {
	t.Parallel()

	optional := fixing.OptionalRules()
	assert.Equal(t, []fixing.Rule{
		fixing.RuleAdCues, fixing.RuleDoubleApostrophes, fixing.RuleLoneLowercaseI, fixing.RuleUnneededPeriods,
		fixing.RuleDialogueOnOneLine, fixing.RuleMusicNotes, fixing.RuleSentenceStartCase,
	}, optional)
	opts := fixing.Options{Language: "en", Enable: optional}
	s := read(t, "fixing/optional-rules.srt")

	_, err := fixing.Apply(s, opts)
	require.NoError(t, err)
	golden := read(t, "fixing/optional-rules.fixed.srt")
	assert.Equal(t, cuesOf(golden), cuesOf(s))
	again, err := fixing.Apply(golden, opts)
	require.NoError(t, err)
	assert.Empty(t, again.Fixes)
}

func TestListsEachFixOfTheWebFileWithTheTextBeforeAndAfter(t *testing.T) {
	t.Parallel()

	report, err := fixing.Apply(read(t, "fixing/web-errors.srt"), fixing.Options{Language: "en"})
	require.NoError(t, err)

	type cueRule struct {
		cue  int
		rule fixing.Rule
	}
	got := make([]cueRule, 0, len(report.Fixes))
	for _, f := range report.Fixes {
		got = append(got, cueRule{f.CueIndex, f.Rule})
	}
	assert.Equal(t, []cueRule{
		{0, fixing.RuleDoubleSpaces},
		{0, fixing.RuleSpaceBeforePunctuation},
		{1, fixing.RuleMissingSpaceAfterPunctuation},
		{2, fixing.RuleUnbalancedTags},
		{3, fixing.RuleEmptyTags},
		{4, fixing.RuleDialogueDashes},
		{5, fixing.RuleEllipsis},
		{7, fixing.RuleSpaceBeforePunctuation},
		{8, fixing.RuleUnbalancedTags},
		{9, fixing.RuleMissingSpaceAfterPunctuation},
		{10, fixing.RuleUnbalancedTags},
	}, got)
	assert.Equal(t, "Hello <i> there</i>, how was the trip ?", report.Fixes[0].Before)
	assert.Equal(t, "Hello <i>there</i>, how was the trip ?", report.Fixes[0].After)
	assert.Equal(t, "Hello <i>there</i>, how was the trip ?", report.Fixes[1].Before)
	assert.Equal(t, "<i>The station was closed,\nso we walked.</i>", report.Fixes[3].After)
}

func TestPreviewListsTheFixesAndChangesNothing(t *testing.T) {
	t.Parallel()

	s := read(t, "fixing/web-errors.srt")
	before := cuesOf(s)
	opts := fixing.Options{Language: "en"}

	preview, err := fixing.Preview(s, opts)
	require.NoError(t, err)
	assert.Equal(t, before, cuesOf(s))
	applied, err := fixing.Apply(read(t, "fixing/web-errors.srt"), opts)
	require.NoError(t, err)
	assert.Equal(t, applied, preview)
}

func TestAllRulesOffChangesNothing(t *testing.T) {
	t.Parallel()

	s := read(t, "fixing/ocr-en.ocr.srt")
	before := cuesOf(s)

	report, err := fixing.Apply(s, withLanguage(allOff(), "en"))
	require.NoError(t, err)
	assert.Empty(t, report.Fixes)
	assert.Equal(t, before, cuesOf(s))
}

func TestTakesTheLanguageFromTheMetadataWithoutTheOption(t *testing.T) {
	t.Parallel()

	s := fromTexts("lt is l.")
	s.SetMetadata(model.MetadataLanguage, "en-GB")
	_, err := fixing.Apply(s, fixing.Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"It is I."}, texts(s))

	_, fixes := fixLines(t, []string{"lt is l."}, fixing.Options{Language: "nl"})
	assert.Empty(t, fixes)
}

func TestDefaultRulesAndStyles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		opts  fixing.Options
		want  []string
	}{
		{"dialogue on one line is off by default", []string{"- Hi. - Hello."}, fixing.Options{Language: "en"}, []string{"- Hi. - Hello."}},
		{
			"dialogue on one line takes the dash style",
			[]string{"Hi. - Hello."},
			fixing.Options{
				Enable: []fixing.Rule{fixing.RuleDialogueOnOneLine}, DialogueDashStyle: model.DialogueDashEnDashSpace,
			},
			[]string{"\u2013 Hi.", "\u2013 Hello."},
		},
		{"lone i is off by default", []string{"i think"}, fixing.Options{Language: "en"}, []string{"i think"}},
		{"sentence start case is off by default", []string{"hello."}, fixing.Options{Language: "en"}, []string{"hello."}},
		{"double apostrophes is off by default", []string{"''Hi''"}, fixing.Options{Language: "en"}, []string{"''Hi''"}},
		{"unneeded periods is off by default", []string{"Really?."}, fixing.Options{Language: "en"}, []string{"Really?."}},
		{"music notes is off by default", []string{"# La la #"}, fixing.Options{Language: "en"}, []string{"# La la #"}},
		{
			"Unicode ellipsis replaces all ellipses",
			[]string{"Well... no. . . yes....", "Wait\u2026"},
			fixing.Options{Language: "en", Ellipsis: fixing.EllipsisUnicode},
			[]string{"Well\u2026 no\u2026 yes\u2026", "Wait\u2026"},
		},
		{"Disable turns the ellipsis off", []string{"Well. . . no."}, fixing.Options{Disable: []fixing.Rule{fixing.RuleEllipsis}}, []string{"Well. . . no."}},
		{
			"hyphen dash style",
			[]string{"- Hi.", "-  Hello."},
			fixing.Options{DialogueDashStyle: model.DialogueDashHyphen},
			[]string{"-Hi.", "-Hello."},
		},
		{"en dash style", []string{"-Hi."}, fixing.Options{DialogueDashStyle: model.DialogueDashEnDashSpace}, []string{"\u2013 Hi."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lines, _ := fixLines(t, tt.lines, tt.opts)
			assert.Equal(t, tt.want, lines)
		})
	}
}

func TestDialogueOnOneLineSplitsAtADashAfterAChineseOrJapaneseSentenceEnd(t *testing.T) {
	t.Parallel()

	opts := fixing.Options{Language: "zh", Enable: []fixing.Rule{fixing.RuleDialogueOnOneLine}}
	s := read(t, "fixes/own_cjk_dialogue.srt")

	_, err := fixing.Apply(s, opts)
	require.NoError(t, err)
	golden := read(t, "fixing/cjk-dialogue.fixed.srt")
	assert.Equal(t, cuesOf(golden), cuesOf(s))
	again, err := fixing.Apply(golden, opts)
	require.NoError(t, err)
	assert.Empty(t, again.Fixes)
}

func TestSentenceStartCaseLooksAtTheCueBefore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, previous, text, want string
	}{
		{"after a full stop", "I'm home.", "where are you?", "Where are you?"},
		{"after a question", "Really?", "<i>yes.</i>", "<i>Yes.</i>"},
		{"after a quote", `"Stop!"`, "fine.", "Fine."},
		{"after an ellipsis", "I was going to...", "stay home.", "stay home."},
		{"after U+2026", "I was going to\u2026", "stay home.", "stay home."},
		{"after a comma", "So I said,", "no way.", "no way."},
		{"inner capital", "Done.", "eBay is here.", "eBay is here."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.previous, tt.text)
			_, err := fixing.Apply(s, withLanguage(only(fixing.RuleSentenceStartCase), "en"))
			require.NoError(t, err)
			assert.Equal(t, []string{tt.previous, tt.want}, texts(s))
		})
	}
}

func TestAdCuesRemovesCuesThatNameADownloadSite(t *testing.T) {
	t.Parallel()

	s := read(t, "fixing/ad-cues.srt")
	report, err := fixing.Apply(s, withLanguage(only(fixing.RuleAdCues), "en"))
	require.NoError(t, err)

	type fix struct {
		cue   int
		after string
	}
	got := make([]fix, 0, len(report.Fixes))
	for _, f := range report.Fixes {
		assert.Equal(t, fixing.RuleAdCues, f.Rule)
		got = append(got, fix{f.CueIndex, f.After})
	}
	assert.Equal(t, []fix{{0, ""}, {5, ""}, {7, ""}}, got)
	assert.Equal(t, []string{
		"Subtitles by Marta Lindqvist", "The ferry leaves at seven.", "Send the photos to ellen.marsh@gmail.com.",
		"Translated by whom? It's full of mistakes.", "We landed late.It was cold.", "Write this down: www.dunmore-rentals.com.",
	}, texts(s))

	s = read(t, "fixing/ad-cues.srt")
	_, err = fixing.Apply(s, fixing.Options{Language: "en"})
	require.NoError(t, err)
	assert.Equal(t, 9, s.Len())
}

// The 50 files hold 120 ad and credit cues and 1,640 other cues in 10 languages. labels.json names the kind of
// each cue.
func TestAdCuesRemovesTheSiteAdsOfTheTestSetAndNoOtherCue(t *testing.T) {
	t.Parallel()

	var labels map[string][]string
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "fixing/ad-cues-test-set/labels.json"), &labels))
	removed := map[string]int{}
	cues := 0
	for file, kinds := range labels {
		s := read(t, filepath.Join("fixing/ad-cues-test-set", file))
		require.Equal(t, len(kinds), s.Len(), file)
		cues += s.Len()
		report, err := fixing.Preview(s, only(fixing.RuleAdCues))
		require.NoError(t, err)
		for _, f := range report.Fixes {
			removed[kinds[f.CueIndex]]++
		}
	}
	assert.Equal(t, 1760, cues)
	assert.Equal(t, map[string]int{"ad-url": 54, "ad-site": 8}, removed)
}

func TestSkipsCuesWithoutLines(t *testing.T) {
	t.Parallel()

	var s model.Subtitle
	s.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second})
	report, err := fixing.Apply(&s, fixing.Options{})
	require.NoError(t, err)
	assert.Empty(t, report.Fixes)
	assert.Equal(t, 1, s.Len())
}

func TestRemovesACueThatTheFixesEmptyAndKeepsItsComments(t *testing.T) {
	t.Parallel()

	s := fromTexts("Keep", "<i>Sync by YIFY</i>", "lt's me")
	require.NoError(t, s.AddComment("ad", 1))
	report, err := fixing.Apply(s, fixing.Options{Language: "en", Enable: []fixing.Rule{fixing.RuleAdCues}})
	require.NoError(t, err)

	assert.Equal(t, []fixing.Fix{
		{CueIndex: 1, Rule: fixing.RuleAdCues, Before: "<i>Sync by YIFY</i>"},
		{CueIndex: 2, Rule: fixing.RuleOCRLowercaseL, Before: "lt's me", After: "It's me"},
	}, report.Fixes)
	assert.Equal(t, []string{"Keep", "It's me"}, texts(s))
	assert.Equal(t, []model.Comment{{BeforeCue: 1, Text: "ad"}}, s.Comments())
}

func TestRules(t *testing.T) {
	t.Parallel()

	all := fixing.Rules()
	assert.Len(t, all, 17)
	assert.Equal(t, fixing.RuleAdCues, all[0])
	assert.Equal(t, fixing.RuleSentenceStartCase, all[16])
	var defaults []fixing.Rule
	for _, r := range all {
		if r.Default() {
			defaults = append(defaults, r)
		}
	}
	assert.Len(t, defaults, 10)
	assert.False(t, fixing.Rule("nope").Default())
	assert.Equal(t, "ocrPipe", fixing.RuleOCRPipe.String())
	assert.Equal(t, "EllipsisUnicode", fixing.EllipsisUnicode.String())
	assert.Equal(t, "EllipsisStyle(7)", fixing.EllipsisStyle(7).String())
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts fixing.Options
		want string
	}{
		{"zero value", fixing.Options{}, ""},
		{"rule that is on in Enable", fixing.Options{Enable: []fixing.Rule{fixing.RuleEllipsis}}, ""},
		{"rule that is off in Disable", fixing.Options{Disable: []fixing.Rule{fixing.RuleAdCues}}, ""},
		{
			"unknown rule in Enable",
			fixing.Options{Enable: []fixing.Rule{"nope"}},
			`stbox: invalid argument: fixing.Options.Enable: Unknown rule "nope".`,
		},
		{
			"unknown rule in Disable",
			fixing.Options{Disable: []fixing.Rule{"OCRPipe"}},
			`stbox: invalid argument: fixing.Options.Disable: Unknown rule "OCRPipe".`,
		},
		{
			"rule in both lists",
			fixing.Options{Enable: []fixing.Rule{fixing.RuleMusicNotes}, Disable: []fixing.Rule{fixing.RuleMusicNotes}},
			"stbox: invalid argument: fixing.Options.Enable: The rule musicNotes is in both Enable and Disable.",
		},
		{
			"dash style outside the constants",
			fixing.Options{DialogueDashStyle: model.DialogueDashStyle(8)},
			"stbox: invalid argument: fixing.Options.DialogueDashStyle: The style DialogueDashStyle(8) is no dialogue dash style.",
		},
		{
			"ellipsis style outside the constants",
			fixing.Options{Ellipsis: fixing.EllipsisStyle(2)},
			"stbox: invalid argument: fixing.Options.Ellipsis: The style EllipsisStyle(2) is not EllipsisThreeDots or EllipsisUnicode.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			assert.Equal(t, tt.want, err.Error())

			s := fromTexts("lt's")
			_, err = fixing.Apply(s, tt.opts)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			_, err = fixing.Preview(s, tt.opts)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			assert.Equal(t, []string{"lt's"}, texts(s))
		})
	}
}
