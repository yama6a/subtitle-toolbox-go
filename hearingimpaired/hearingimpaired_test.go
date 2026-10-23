package hearingimpaired_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/hearingimpaired"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// fromTexts gives one cue per text, the cue i from i s to i+1 s.
func fromTexts(texts ...string) *model.Subtitle {
	var s model.Subtitle
	for i, text := range texts {
		c := model.Cue{Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second}
		c.SetText(text)
		s.AddCue(c)
	}
	return &s
}

func texts(s *model.Subtitle) []string {
	out := []string{}
	for _, c := range s.All() {
		out = append(out, c.Text())
	}
	return out
}

type row struct {
	name string
	in   []string
	opts hearingimpaired.Options
	want []string
}

func runRows(t *testing.T, rows []row) {
	t.Helper()

	for _, tt := range rows {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTexts(tt.in...)
			_, err := hearingimpaired.Apply(s, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.want, texts(s))
		})
	}
}

func one(text string) []string { return []string{text} }

func load(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, rel), stbox.ReadOptions{})
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

func assertSameCues(t *testing.T, want, got *model.Subtitle) {
	t.Helper()

	assert.Equal(t, cuesOf(want), cuesOf(got))
	assert.Equal(t, want.Comments(), got.Comments())
}

func assertWrites(t *testing.T, s *model.Subtitle, f stbox.Format, opts stbox.WriteOptions, want string) {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, stbox.Write(&buf, s, f, opts))
	assert.Equal(t, string(testfiles.Read(t, want)), buf.String())
}

var allOptions = hearingimpaired.Options{
	AnyCaseLabels:  true,
	CustomBrackets: []hearingimpaired.BracketPair{{Open: "{", Close: "}"}},
	Lyrics:         true,
}

// Adapted from HearingImpairedRemoverTest::testRealSubRipFileParsesAndRoundTrips.
func TestRealSubRipFileRoundTrips(t *testing.T) {
	t.Parallel()

	content := testfiles.Read(t, "hearing-impaired/own_sdh.srt")
	s, _, err := stbox.Parse(bytes.NewReader(content), stbox.ReadOptions{Format: stbox.SubRip})
	require.NoError(t, err)
	cues := s.Cues()
	require.Len(t, cues, 14)
	assert.Equal(t, cueLines{time.Second, 3 * time.Second, one("[TRAIN WHISTLE BLOWS]")}, cuesOf(s)[0])
	assert.Equal(t, cueLines{
		39500 * time.Millisecond, 42 * time.Second,
		[]string{"- # The wheels go round #", "- It stopped raining. (birds chirping)"},
	}, cuesOf(s)[13])

	var buf bytes.Buffer
	require.NoError(t, stbox.Write(&buf, s, stbox.SubRip, stbox.WriteOptions{LineEnding: stbox.CRLF, BOM: stbox.BOMAdd}))
	assert.Equal(t, string(content), buf.String())
}

// Adapted from HearingImpairedRemoverTest::testRealSubRipFileWithDefaultOptions and
// HearingImpairedRemoverTest::testRealSubRipFileWithAllOptions.
func TestApplyRealSubRipFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		opts   hearingimpaired.Options
		write  stbox.WriteOptions
		want   string
		report hearingimpaired.Report
	}{
		{
			"defaults",
			hearingimpaired.Options{},
			stbox.WriteOptions{LineEnding: stbox.CRLF, BOM: stbox.BOMAdd},
			"hearing-impaired/own_sdh_removed.srt",
			hearingimpaired.Report{RemovedLines: 6, RemovedCues: 3},
		},
		{
			"all options", allOptions,
			stbox.WriteOptions{},
			"hearing-impaired/own_sdh_removed_all_options.srt",
			hearingimpaired.Report{RemovedLines: 8, RemovedCues: 4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := load(t, "hearing-impaired/own_sdh.srt")
			report, err := hearingimpaired.Apply(s, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.report, report)
			assertSameCues(t, load(t, tt.want), s)
			assertWrites(t, s, stbox.SubRip, tt.write, tt.want)
		})
	}
}

// Adapted from HearingImpairedRemoverTest::testRealWebVttFileParsesAndRoundTrips.
func TestRealWebVTTFileRoundTrips(t *testing.T) {
	t.Parallel()

	s := load(t, "hearing-impaired/own_sdh.vtt")
	require.Equal(t, 8, s.Len())
	assert.Equal(t, cueLines{time.Second, 3 * time.Second, one("[wind howling]")}, cuesOf(s)[0])
	assert.Equal(t, cueLines{21500 * time.Millisecond, 24 * time.Second, []string{"<i>[door opens]", "(footsteps)</i>"}}, cuesOf(s)[7])

	var buf bytes.Buffer
	require.NoError(t, stbox.Write(&buf, s, stbox.WebVTT, stbox.WriteOptions{}))
	again, _, err := stbox.Parse(&buf, stbox.ReadOptions{Format: stbox.WebVTT})
	require.NoError(t, err)
	assertSameCues(t, s, again)
}

// Adapted from HearingImpairedRemoverTest::testRealWebVttFileWithDefaultOptions.
func TestApplyRealWebVTTFile(t *testing.T) {
	t.Parallel()

	s := load(t, "hearing-impaired/own_sdh.vtt")
	report, err := hearingimpaired.Apply(s, hearingimpaired.Options{})
	require.NoError(t, err)
	assert.Equal(t, hearingimpaired.Report{RemovedLines: 6, RemovedCues: 3}, report)
	assertSameCues(t, load(t, "hearing-impaired/own_sdh_removed.vtt"), s)
	assertWrites(t, s, stbox.WebVTT, stbox.WriteOptions{}, "hearing-impaired/own_sdh_removed.vtt")
}

// Adapted from HearingImpairedRemoverTest::testExamplesOfTheIssue and testRemovesSpacesAroundAnAnnotation.
func TestApplyExamples(t *testing.T) {
	t.Parallel()

	runRows(t, []row{
		{"square brackets", one("[DOOR SLAMS]"), hearingimpaired.Options{}, []string{}},
		{"parentheses", one("(laughs) You came back."), hearingimpaired.Options{}, one("You came back.")},
		{"speaker label", one("JOHN: Where were you?"), hearingimpaired.Options{}, one("Where were you?")},
		{"dialogue", one("- [gasps] No!\n- MARY: Yes."), hearingimpaired.Options{}, one("- No!\n- Yes.")},
		{"lyrics kept", one("# Never gonna stop #"), hearingimpaired.Options{}, one("# Never gonna stop #")},
		{"lyrics removed", one("# Never gonna stop #"), hearingimpaired.Options{Lyrics: true}, []string{}},
		{"inside italics", one("<i>(whispering) Over here.</i>"), hearingimpaired.Options{}, one("<i>Over here.</i>")},
		{"space after", one("Wait (sighs) for it."), hearingimpaired.Options{}, one("Wait for it.")},
		{"space before a dot", one("Wait for it (sighs)."), hearingimpaired.Options{}, one("Wait for it.")},
		{"at the end", one("Wait. [door closes]"), hearingimpaired.Options{}, one("Wait.")},
		{"after a dash", one("-[gasps] No!"), hearingimpaired.Options{}, one("- No!")},
		{"no space before a word", one("ab(x)cd"), hearingimpaired.Options{}, one("abcd")},
		{"space before only", one("ab (x)cd"), hearingimpaired.Options{}, one("ab cd")},
		{"space after only", one("ab(x) cd"), hearingimpaired.Options{}, one("ab cd")},
		{"comma after", one("ab (x), cd"), hearingimpaired.Options{}, one("ab, cd")},
		{"tabs before a dot", one("ab\t(x)\t."), hearingimpaired.Options{}, one("ab.")},
	})
}

// Adapted from HearingImpairedRemoverTest::testBracketsAcrossTagsAndLines.
func TestApplyBracketsAcrossTagsAndLines(t *testing.T) {
	t.Parallel()

	runRows(t, []row{
		{"tag around", one("<font color=\"#ff0000\">[BELL]</font> Hello."), hearingimpaired.Options{}, one("Hello.")},
		{"tag inside", one("[BELL <i>RINGS</i>] Hello."), hearingimpaired.Options{}, one("Hello.")},
		{"two lines", one("(woman\nspeaking) Hi."), hearingimpaired.Options{}, one("Hi.")},
		{"colon after", one("(MARY): Hi."), hearingimpaired.Options{}, one("Hi.")},
		{"colon before a word", one("Hi (a):b"), hearingimpaired.Options{}, one("Hi b")},
		{"nested", one("[[echo]] Hi."), hearingimpaired.Options{}, one("Hi.")},
		{"not closed", one("Hi (there"), hearingimpaired.Options{}, one("Hi (there")},
		{"crossed pairs", one("(a [b) c]"), hearingimpaired.Options{}, []string{}},
		{"crossed pairs reversed", one("[a (b] c)"), hearingimpaired.Options{}, []string{}},
	})
}

// Adapted from HearingImpairedRemoverTest::testKeepsTagsAndEntities.
func TestApplyKeepsTagsAndEntities(t *testing.T) {
	t.Parallel()

	runRows(t, []row{
		{"first line goes", one("<i>[sighs]\nHello</i>"), hearingimpaired.Options{}, one("<i>Hello</i>")},
		{"last line goes", one("<i>Hello\n[sighs]</i>"), hearingimpaired.Options{}, one("<i>Hello</i>")},
		{"voice tag", one("<v Anna>(coughs) Good morning."), hearingimpaired.Options{}, one("<v Anna>Good morning.")},
		{"entities", one("Fish &amp; (chips) &lt;3"), hearingimpaired.Options{}, one("Fish &amp; &lt;3")},
		{"no annotation", one("Hi &amp; bye"), hearingimpaired.Options{}, one("Hi &amp; bye")},
		{"second dialogue line", one("- Hi.\n- (laughs) Bye."), hearingimpaired.Options{}, one("- Hi.\n- Bye.")},
		{"empty pair with spaces", one("<i> [y] </i>\nHi"), hearingimpaired.Options{}, one("Hi")},
		{"nested empty pairs", one("<b><i>[y]</i></b>\nHi"), hearingimpaired.Options{}, one("Hi")},
		{"space before the end of a tag", one("<i >[y]</i >\nHi"), hearingimpaired.Options{}, one("Hi")},
		{"tag names compare with case", one("<I>[y]</i>\nHi"), hearingimpaired.Options{}, one("<I></i>Hi")},
		{"U+00A0 in a blank pair", one("<i>[x]\u00a0</i>\nHi"), hearingimpaired.Options{}, one("Hi")},
		{"U+00A0 in a blank pair after text", one("Hi [x]<i>\u00a0</i>"), hearingimpaired.Options{}, one("Hi")},
		{"empty pair of an unchanged line", one("(x)\n<i></i>Hi"), hearingimpaired.Options{}, one("<i></i>Hi")},
		{"spaces around an empty pair", one("x <i></i> [y]"), hearingimpaired.Options{}, one("x")},
	})
}

// Adapted from HearingImpairedRemoverTest::testSpeakerLabels.
func TestApplySpeakerLabels(t *testing.T) {
	t.Parallel()

	runRows(t, []row{
		{"number", one("MAN 2: Run!"), hearingimpaired.Options{}, one("Run!")},
		{"dot and apostrophe", one("DR. O'NEIL: Sit."), hearingimpaired.Options{}, one("Sit.")},
		{"accented", one("\u00c9MILE: Hi."), hearingimpaired.Options{}, one("Hi.")},
		{"mixed case", one("Note: this stays."), hearingimpaired.Options{}, one("Note: this stays.")},
		{"time", one("THE 8:15 TRAIN"), hearingimpaired.Options{}, one("THE 8:15 TRAIN")},
		{"digits only", one("12: Go"), hearingimpaired.Options{}, one("12: Go")},
		{"own line", one("JOHN:\nWhere?"), hearingimpaired.Options{}, one("Where?")},
		{"any case", one("Baker: Hi."), hearingimpaired.Options{AnyCaseLabels: true}, one("Hi.")},
		{"kept", one("JOHN: Hi."), hearingimpaired.Options{KeepSpeakerLabels: true}, one("JOHN: Hi.")},
		{"U+00A0 before", one("\u00a0JOHN: Hi."), hearingimpaired.Options{}, one("\u00a0Hi.")},
		{"U+180E before", one("\u180eJOHN: Hi."), hearingimpaired.Options{}, one("\u180eHi.")},
		{"U+00A0 after", one("JOHN:\u00a0Hi."), hearingimpaired.Options{}, one("Hi.")},
		{"space before the colon", one("JOHN :Hi."), hearingimpaired.Options{}, one("JOHN :Hi.")},
		{"two dashes", one("- -JOHN: Hi"), hearingimpaired.Options{}, one("- -JOHN: Hi")},
	})
}

// Adapted from HearingImpairedRemoverTest::testMusicLines.
func TestApplyMusicLines(t *testing.T) {
	t.Parallel()

	runRows(t, []row{
		{"notes", one("\u266a \u266a"), hearingimpaired.Options{}, []string{}},
		{"dash and hash", one("- #"), hearingimpaired.Options{}, []string{}},
		{"hash", one("#"), hearingimpaired.Options{}, []string{}},
		{"note line", one("\u266b\nHi."), hearingimpaired.Options{}, one("Hi.")},
		{"lyrics and a number", []string{"\u266a Sing along \u266a", "#1 fan"}, hearingimpaired.Options{}, []string{"\u266a Sing along \u266a", "#1 fan"}},
		{"lyrics removed", []string{"\u266a Sing along \u266a", "#1 fan"}, hearingimpaired.Options{Lyrics: true}, one("#1 fan")},
		{"lyrics over 2 lines", one("\u266a The rain\nkeeps falling \u266a\nHi."), hearingimpaired.Options{Lyrics: true}, one("Hi.")},
		{"kept", one("\u266a \u266a"), hearingimpaired.Options{KeepMusicLines: true}, one("\u266a \u266a")},
		{"kept hash", one("- #"), hearingimpaired.Options{KeepMusicLines: true}, one("- #")},
		{"U+180E between notes", one("\u266a\u180e\u266a"), hearingimpaired.Options{}, []string{}},
		{"hash before U+180E", one("#\u180e"), hearingimpaired.Options{}, []string{}},
		{"hash before U+0085", one("#\u0085"), hearingimpaired.Options{}, one("#\u0085")},
		{"U+3000 after", one("Hi\n- # \u3000"), hearingimpaired.Options{}, one("Hi")},
		{"hash after U+180E", one("a\u180e# b # c"), hearingimpaired.Options{Lyrics: true}, one("a\u180e c")},
		{"hash after U+0085", one("a\u0085# b # c"), hearingimpaired.Options{Lyrics: true}, one("a\u0085 c")},
		{"hash after U+200B", one("a\u200b# b # c"), hearingimpaired.Options{Lyrics: true}, one("a\u200b# b # c")},
		{"line ends with a hash", one("Hi #"), hearingimpaired.Options{Lyrics: true}, []string{}},
		{"line ends with a hash and U+00A0", one("Hi #\u00a0"), hearingimpaired.Options{Lyrics: true}, []string{}},
		{"hash after a dash", one("Hi -#"), hearingimpaired.Options{Lyrics: true}, one("Hi -#")},
		{"hash before a word", one("- #Hi"), hearingimpaired.Options{Lyrics: true}, one("- #Hi")},
		{"two hashes", one("Hi ##"), hearingimpaired.Options{Lyrics: true}, one("Hi ##")},
	})
}

// Adapted from HearingImpairedRemoverTest::testDialogueDashes.
func TestApplyDialogueDashes(t *testing.T) {
	t.Parallel()

	runRows(t, []row{
		{"one line left", one("- Is it open?\n- (laughs)"), hearingimpaired.Options{}, one("Is it open?")},
		{"label line", one("- JOHN:\n- Yes?"), hearingimpaired.Options{}, one("Yes?")},
		{"two lines left", one("- A.\n- [sighs]\n- B."), hearingimpaired.Options{}, one("- A.\n- B.")},
		{"no line removed", one("- Is it open? (laughs)"), hearingimpaired.Options{}, one("- Is it open?")},
		{"U+00A0 before the dash", one("\u00a0- Is it open?\n- (laughs)"), hearingimpaired.Options{}, one("Is it open?")},
		{"U+2000 before the dash", one("\u2000- A.\n- [x]"), hearingimpaired.Options{}, one("A.")},
		{"U+2000 after the dash", one("-\u2000A.\n- [x]"), hearingimpaired.Options{}, one("A.")},
		{"U+00A0 and a dash is no empty line", one("- A.\n\u00a0- [x]"), hearingimpaired.Options{}, one("- A.\n\u00a0-")},
		{"dash in a tag", one("<i>- A.</i>\n- [x]"), hearingimpaired.Options{}, one("<i>A.</i>")},
	})
}

// Adapted from HearingImpairedRemoverTest::testCustomBrackets.
func TestApplyCustomBrackets(t *testing.T) {
	t.Parallel()

	braces := []hearingimpaired.BracketPair{{Open: "{", Close: "}"}}
	runRows(t, []row{
		{"not by default", one("Hi {laughs}"), hearingimpaired.Options{}, one("Hi {laughs}")},
		{"ASS override stays", one("{\\an8}Hi {laughs}"), hearingimpaired.Options{CustomBrackets: braces}, one("{\\an8}Hi")},
		{"backslash inside", one("{a\\b} {c}"), hearingimpaired.Options{CustomBrackets: braces}, []string{}},
		{"backslash first", one("{\\a} {c}"), hearingimpaired.Options{CustomBrackets: braces}, one("{\\a}")},
		{"same string twice", one("Hi *sighs*"), hearingimpaired.Options{CustomBrackets: []hearingimpaired.BracketPair{{Open: "*", Close: "*"}}}, one("Hi")},
		{"two pairs of the same string", one("*a* *b*"), hearingimpaired.Options{CustomBrackets: []hearingimpaired.BracketPair{{Open: "*", Close: "*"}}}, []string{}},
		{"longer strings", one("Hi <<laughs>> there"), hearingimpaired.Options{CustomBrackets: []hearingimpaired.BracketPair{{Open: "&lt;&lt;", Close: "&gt;&gt;"}}}, one("Hi <<laughs>> there")},
		{"decoded strings", one("Hi &lt;&lt;laughs&gt;&gt; there"), hearingimpaired.Options{CustomBrackets: []hearingimpaired.BracketPair{{Open: "<<", Close: ">>"}}}, one("Hi there")},
		{"all kept", one("[A] (B)"), hearingimpaired.Options{KeepSquareBrackets: true, KeepParentheses: true}, one("[A] (B)")},
		{"square brackets kept", one("[A] (B)"), hearingimpaired.Options{KeepSquareBrackets: true}, one("[A]")},
		{"parentheses kept", one("[A] (B)"), hearingimpaired.Options{KeepParentheses: true}, one("(B)")},
	})
}

// Adapted from HearingImpairedRemoverTest::testKeepsInvalidUtf8.
func TestApplyKeepsInvalidUTF8(t *testing.T) {
	t.Parallel()

	runRows(t, []row{
		{"Latin-1", one("\xe9t\xe9 [x]"), hearingimpaired.Options{}, one("\xe9t\xe9 [x]")},
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pair hearingimpaired.BracketPair
		ok   bool
	}{
		{"braces", hearingimpaired.BracketPair{Open: "{", Close: "}"}, true},
		{"empty open", hearingimpaired.BracketPair{Close: "}"}, false},
		{"empty close", hearingimpaired.BracketPair{Open: "{"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := hearingimpaired.Options{CustomBrackets: []hearingimpaired.BracketPair{tt.pair}}
			err := opts.Validate()
			s := fromTexts("[x] {y}")
			report, applyErr := hearingimpaired.Apply(s, opts)
			_, isErr := hearingimpaired.IsAnnotation("[x]", opts)
			if tt.ok {
				require.NoError(t, err)
				require.NoError(t, applyErr)
				require.NoError(t, isErr)
				return
			}
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var aerr *model.InvalidArgumentError
			require.ErrorAs(t, err, &aerr)
			assert.Equal(t, `Each custom bracket pair must hold two non-empty strings, for example "{" and "}".`, aerr.Msg)
			require.ErrorIs(t, applyErr, model.ErrInvalidArgument)
			require.ErrorIs(t, isErr, model.ErrInvalidArgument)
			assert.Equal(t, hearingimpaired.Report{}, report)
			assert.Equal(t, one("[x] {y}"), texts(s))
		})
	}
}

// Adapted from HearingImpairedRemoverTest::testRemovesEmptyCuesAndKeepsComments.
func TestApplyRemovesEmptyCuesAndKeepsComments(t *testing.T) {
	t.Parallel()

	s := fromTexts("first", "<i>[MUSIC]</i>", "[DOOR]\n(SIGHS)", "last")
	s.AddCue(model.Cue{Start: 10 * time.Second, End: 11 * time.Second})
	require.NoError(t, s.AddComment("before music", 1))
	require.NoError(t, s.AddComment("before door", 2))
	require.NoError(t, s.AddComment("before last", 3))

	report, err := hearingimpaired.Apply(s, hearingimpaired.Options{})
	require.NoError(t, err)

	assert.Equal(t, hearingimpaired.Report{RemovedLines: 3, RemovedCues: 2}, report)
	assert.Equal(t, []string{"first", "last", ""}, texts(s))
	assert.Equal(t, []model.Comment{
		{Text: "before music", BeforeCue: 1},
		{Text: "before door", BeforeCue: 1},
		{Text: "before last", BeforeCue: 1},
	}, s.Comments())
}

func TestApplyCountsRemovedLinesOfKeptCues(t *testing.T) {
	t.Parallel()

	s := fromTexts("[a]\nHi\n(b)", "Hi [c]")
	report, err := hearingimpaired.Apply(s, hearingimpaired.Options{})
	require.NoError(t, err)
	assert.Equal(t, hearingimpaired.Report{RemovedLines: 2}, report)
	assert.Equal(t, []string{"Hi", "Hi"}, texts(s))
}

// Adapted from HearingImpairedRemoverTest::testIsAnnotation.
func TestIsAnnotation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line string
		opts hearingimpaired.Options
		want bool
	}{
		{"[DOOR SLAMS]", hearingimpaired.Options{}, true},
		{"JOHN: Hi.", hearingimpaired.Options{}, true},
		{"\u266a \u266a", hearingimpaired.Options{}, true},
		{"Note: this stays.", hearingimpaired.Options{}, false},
		{"\u266a Sing along \u266a", hearingimpaired.Options{}, false},
		{"\u266a Sing along \u266a", hearingimpaired.Options{Lyrics: true}, true},
		{"  Hi  there ", hearingimpaired.Options{}, false},
		{"", hearingimpaired.Options{}, false},
		{"a\rb", hearingimpaired.Options{}, false},
		{"a  \r  b", hearingimpaired.Options{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()

			got, err := hearingimpaired.IsAnnotation(tt.line, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
