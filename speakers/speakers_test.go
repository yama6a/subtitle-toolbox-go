package speakers_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/speakers"
)

// view is what the tests compare of a cue.
type view struct {
	Start, End time.Duration
	Speaker    string
	Lines      []string
}

func viewsOf(s *model.Subtitle) []view {
	var out []view
	for _, c := range s.All() {
		out = append(out, view{c.Start, c.End, c.Speaker, c.Lines})
	}
	return out
}

func load(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, rel), stbox.ReadOptions{})
	require.NoError(t, err)
	return s
}

// cueOf reads text as a reader does: the lines, then the voice that covers the whole cue becomes Speaker.
func cueOf(text string) model.Cue {
	c := model.Cue{End: time.Second}
	c.SetText(text)
	c.ExtractWordTags()
	return c
}

func subtitleOf(texts ...string) *model.Subtitle {
	var s model.Subtitle
	for i, text := range texts {
		c := cueOf(text)
		c.Start, c.End = time.Duration(i)*time.Second, time.Duration(i+1)*time.Second
		s.AddCue(c)
	}
	return &s
}

// vttData is the vtt cue format data of a cue that came from a WebVTT file. Its writer keeps the voice class only then.
func vttData() *model.Data {
	d := model.NewData()
	d.Set("voiceEnd", true)
	return d
}

func ptr(s string) *string { return &s }

// Adapted from SpeakerLabelsTest::testToPrefixFile, testToDialogueDashesFile, testToColoursFile and testFromPrefixFile.
func TestApplyFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, in, want string
		opts           speakers.Options
		changed        int
	}{
		{"prefix", "speakers/voices.vtt", "speakers/voices_prefix.srt", speakers.Options{Style: speakers.StylePrefix}, 6},
		{"dashes", "speakers/voices.vtt", "speakers/voices_dashes.srt", speakers.Options{Style: speakers.StyleDialogueDashes}, 6},
		{"colors", "speakers/voices.vtt", "speakers/voices_colors.srt", speakers.Options{Style: speakers.StyleColors}, 6},
		{"from labels", "speakers/sdh_labels.srt", "speakers/sdh_labels_voices.vtt", speakers.Options{FromLabels: true}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := load(t, tt.in)
			report, err := speakers.Apply(s, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.changed, report.ChangedCues)
			assert.Equal(t, viewsOf(load(t, tt.want)), viewsOf(s))
		})
	}
}

// Adapted from SpeakerLabelsTest::testVoicesFileParsesAndRoundTrips.
func TestVoicesFileHoldsSpeakers(t *testing.T) {
	t.Parallel()

	got := viewsOf(load(t, "speakers/voices.vtt"))
	require.Len(t, got, 7)
	assert.Equal(t, view{time.Second, 3200 * time.Millisecond, "Anna", []string{"Is the ferry on time today?"}}, got[0])
	assert.Equal(t, view{15200 * time.Millisecond, 17400 * time.Millisecond, "Clara", []string{"We meet again next week."}}, got[6])
	assert.Equal(t, []string{"<v.host Clara>Welcome back to the show. <v Anna>Thank you, Clara."}, got[3].Lines)
}

type want struct {
	speaker string
	lines   []string
}

// Adapted from SpeakerLabelsTest::testFromPrefix and its provider fromPrefixCases.
func TestFromLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, in string
		anyCase  bool
		want     want
	}{
		{"upper case label", "JOHN: Hi.", false, want{"John", []string{"Hi."}}},
		{"title, dot and quote", "DR. O'NEIL: Yes.", false, want{"Dr. O'Neil", []string{"Yes."}}},
		{"number", "MAN 2: Run!", false, want{"Man 2", []string{"Run!"}}},
		{"accented label", "ÉMILE: Salut.", false, want{"Émile", []string{"Salut."}}},
		{"mixed case label", "Note: this stays.", false, want{"", []string{"Note: this stays."}}},
		{"mixed case allowed", "Baker: Hi.", true, want{"Baker", []string{"Hi."}}},
		{"dialogue dashes", "- JOHN: Hi.\n- MARY: Bye.", false, want{"", []string{"<v John>Hi.", "<v Mary>Bye."}}},
		{"inside italics", "<i>JOHN: Hi.</i>", false, want{"John", []string{"<i>Hi.</i>"}}},
		{"label on its own line", "JOHN:\nHi there.", false, want{"John", []string{"Hi there."}}},
		{"label alone in its own tag pair", "<i>JOHN:</i>\nHi.", false, want{"John", []string{"Hi."}}},
		{"label on its own line keeps its style tags", "<i>JOHN:\nMARY: Hi.</i>", false, want{"Mary", []string{"<i>Hi.</i>"}}},
		{"no-break space before the dash", "\u00a0- JOHN: Hi.", false, want{"John", []string{"Hi."}}},
		{"label is the whole cue", "JOHN:", false, want{"", []string{"JOHN:"}}},
		{"no space after colon", "JOHN:Hi.", false, want{"", []string{"JOHN:Hi."}}},
		{"label later in the line", "Then JOHN: Hi.", false, want{"", []string{"Then JOHN: Hi."}}},
		{"time in the line", "At 10:30 we go.", false, want{"", []string{"At 10:30 we go."}}},
		{"escaped text", "JOHN: Fish &amp; chips.", false, want{"John", []string{"Fish &amp; chips."}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleOf(tt.in)
			_, err := speakers.Apply(s, speakers.Options{FromLabels: true, AnyCaseLabels: tt.anyCase})
			require.NoError(t, err)
			c, _ := s.Cue(0)
			assert.Equal(t, tt.want, want{c.Speaker, c.Lines})
		})
	}
}

// Adapted from SpeakerLabelsTest::testToPrefix, testToDialogueDashes and testToColoursWithOwnColours.
func TestStyles(t *testing.T) {
	t.Parallel()

	prefix := speakers.Options{Style: speakers.StylePrefix}
	dashes := speakers.Options{Style: speakers.StyleDialogueDashes}
	tests := []struct {
		name, in string
		opts     speakers.Options
		want     []string
	}{
		{"prefix upper case", "<v Anna>Where were you?", prefix, []string{"ANNA: Where were you?"}},
		{"prefix name as it is", "<v Anna>Where were you?", speakers.Options{Style: speakers.StylePrefix, KeepNameCase: true}, []string{"Anna: Where were you?"}},
		{"prefix own separator", "<v Anna>Where?", speakers.Options{Style: speakers.StylePrefix, Separator: ptr(" - ")}, []string{"ANNA - Where?"}},
		{"prefix empty separator", "<v Anna>Where?", speakers.Options{Style: speakers.StylePrefix, Separator: ptr("")}, []string{"ANNAWhere?"}},
		{"prefix accented name", "<v Zoë>Ja.", prefix, []string{"ZOË: Ja."}},
		{"prefix escaped name", "<v Tom &amp; Jerry>Hi.", prefix, []string{"TOM &amp; JERRY: Hi."}},
		{"prefix speaker over two lines", "<v Anna>Where are\nyou going?", prefix, []string{"ANNA: Where are", "you going?"}},
		{"prefix change in the middle", "<v Anna>Where? <v Ben>Home.", prefix, []string{"ANNA: Where?", "BEN: Home."}},
		{"prefix change inside italics", "<v Anna><i>Where? <v Ben>Home.</i>", prefix, []string{"ANNA: <i>Where?</i>", "BEN: <i>Home.</i>"}},
		{"prefix text after the end tag", "<v Anna>Hi</v> there", prefix, []string{"ANNA: Hi", "there"}},
		{"prefix same speaker twice", "<v Anna>Hi.\n<v Anna>Again.", prefix, []string{"ANNA: Hi.", "Again."}},
		{"prefix no speaker", "Hi <b>there</b>.", prefix, []string{"Hi <b>there</b>."}},
		{"dashes two lines", "<v Anna>Where?\n<v Ben>Home.", dashes, []string{"- Where?", "- Home."}},
		{"dashes one line", "<v Anna>Where? <v Ben>Home.", dashes, []string{"- Where?", "- Home."}},
		{"dashes one speaker", "<v Anna>Where are\nyou going?", dashes, []string{"Where are", "you going?"}},
		{"dashes text without speaker", "Where?\n<v Ben>Home.", dashes, []string{"- Where?", "- Home."}},
		{"dashes dash already there", "<v Anna>- Where?\n<v Ben>Home.", dashes, []string{"- Where?", "- Home."}},
		{"dashes own dash", "<v Anna>Where?\n<v Ben>Home.", speakers.Options{Style: speakers.StyleDialogueDashes, DialogueDashStyle: model.DialogueDashHyphen}, []string{"-Where?", "-Home."}},
		{"dashes three speakers", "<v Anna>One.\n<v Ben>Two.\n<v Clara>Three.", dashes, []string{"- One.", "- Two.", "- Three."}},
		{
			"prefix 5 nested tags, only the 4 innermost reopen", `<b><i><u><font color="#fff"><c.x>Where? <v Ben>Home.</c></font></u></i></b>`, prefix,
			[]string{`<b><i><u><font color="#fff"><c.x>Where?</c></font></u></i>`, `BEN: <i><u><font color="#fff"><c.x>Home.</c></font></u></i></b>`},
		},
		{"prefix text with a less-than sign", "<v Anna>1 < 2", prefix, []string{"ANNA: 1 &lt; 2"}},
		{"prefix voice tag joins text through an empty pair", "<v <i></i></v>Anna>Hi", prefix, []string{"&lt;v <i></i>Anna>Hi"}},
		{"prefix invalid UTF-8 name", "<v anna\xff>Hi.", prefix, []string{"ANNA\xff: Hi."}},
		{"prefix voice tag joins text", "<v </v>Anna>Hi", prefix, []string{"&lt;v Anna>Hi"}},
		{"dashes voice tag joins text", "<v </v>Anna>Hi\nthere", dashes, []string{"&lt;v Anna>Hi", "there"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleOf(tt.in)
			_, err := speakers.Apply(s, tt.opts)
			require.NoError(t, err)
			c, _ := s.Cue(0)
			assert.Equal(t, want{"", tt.want}, want{c.Speaker, c.Lines})
		})
	}
}

// Adapted from SpeakerLabelsTest::testToColoursUsesTheBbcOrderAndStartsAgainAfterTheLastColour and testToColoursWithOwnColours.
func TestColors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		in    []string
		opts  speakers.Options
		wants [][]string
	}{
		{
			"BBC order, then again from the first color",
			[]string{"<v A>1", "<v B>2", "<v C>3", "<v D>4", "<v E>5", "<v A>6 <v B>7", "No speaker."},
			speakers.Options{Style: speakers.StyleColors},
			[][]string{
				{`<font color="#ffffff">1</font>`},
				{`<font color="#ffff00">2</font>`},
				{`<font color="#00ffff">3</font>`},
				{`<font color="#00ff00">4</font>`},
				{`<font color="#ffffff">5</font>`},
				{`<font color="#ffffff">6</font>`, `<font color="#ffff00">7</font>`},
				{"No speaker."},
			},
		},
		{
			"own colors in lower case",
			[]string{"<v A>Hi.\nthere", "No speaker.", "<v B>Bye."},
			speakers.Options{Style: speakers.StyleColors, Colors: []string{"#FF0000", "#00ff00"}},
			[][]string{
				{`<font color="#ff0000">Hi.</font>`, `<font color="#ff0000">there</font>`},
				{"No speaker."},
				{`<font color="#00ff00">Bye.</font>`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleOf(tt.in...)
			_, err := speakers.Apply(s, tt.opts)
			require.NoError(t, err)
			got := make([][]string, 0, s.Len())
			for _, v := range viewsOf(s) {
				got = append(got, v.Lines)
			}
			assert.Equal(t, tt.wants, got)
		})
	}
}

// Adapted from SpeakerLabelsTest::testRenameChangesOnlyTheGivenSpeakers.
func TestRename(t *testing.T) {
	t.Parallel()

	s := subtitleOf("<v SPEAKER_00>Where?", "<v.loud SPEAKER_01>Home.</v>", "<v SPEAKER_02>Here.", "<v SPEAKER_04>Never.")
	opts := speakers.Options{Rename: map[string]string{
		"SPEAKER_00": "Anna", "SPEAKER_01": "O'Neil & Son", "SPEAKER_02": `Sam "Ace" Reed`, "SPEAKER_03": "Nobody",
	}}

	report, err := speakers.Apply(s, opts)
	require.NoError(t, err)

	assert.Equal(t, speakers.Report{ChangedCues: 3}, report)
	assert.Equal(t, []speakers.SpeakerCount{{"Anna", 1}, {"O'Neil & Son", 1}, {`Sam "Ace" Reed`, 1}, {"SPEAKER_04", 1}}, speakers.List(s))
}

func TestVoiceClassSurvivesRenameAndStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		opts      speakers.Options
		speaker   string
		lines     []string
		taggedOut []string
	}{
		{"rename", speakers.Options{Rename: map[string]string{"SPEAKER_01": "O'Neil & Son"}}, "O'Neil & Son", []string{"Home."}, []string{"<v.loud O'Neil &amp; Son>Home.</v>"}},
		{"prefix", speakers.Options{Style: speakers.StylePrefix}, "", []string{"SPEAKER_01: Home."}, []string{"SPEAKER_01: Home."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var s model.Subtitle
			c := model.Cue{End: time.Second, Lines: []string{"<v.loud SPEAKER_01>Home.</v>"}}
			require.NoError(t, c.SetFormatData("vtt", vttData()))
			c.ExtractWordTags()
			require.Equal(t, "SPEAKER_01", c.Speaker)
			s.AddCue(c)

			_, err := speakers.Apply(&s, tt.opts)
			require.NoError(t, err)

			got, _ := s.Cue(0)
			assert.Equal(t, want{tt.speaker, tt.lines}, want{got.Speaker, got.Lines})
			assert.Equal(t, tt.taggedOut, got.TaggedLines())
		})
	}
}

func TestStyleKeepsWordTiming(t *testing.T) {
	t.Parallel()

	var s model.Subtitle
	s.AddCue(model.Cue{
		End:     2 * time.Second,
		Speaker: "Anna",
		Lines:   []string{"Hi there"},
		Words:   []model.Word{{Text: "Hi", Start: time.Second, End: 1500 * time.Millisecond}, {Text: "there", Start: 1500 * time.Millisecond, End: 2 * time.Second}},
	})

	_, err := speakers.Apply(&s, speakers.Options{Style: speakers.StylePrefix})
	require.NoError(t, err)

	c, _ := s.Cue(0)
	assert.Equal(t, []string{"ANNA: Hi there"}, c.Lines)
	assert.Equal(t, []model.Word{{Text: "Hi", Start: time.Second, End: 1500 * time.Millisecond}, {Text: "there", Start: 1500 * time.Millisecond, End: 2 * time.Second}}, c.Words)
}

// Adapted from SpeakerLabelsTest::testFromPrefixAndToPrefixRoundTrip.
func TestFromLabelsThenPrefixRestoresTheLabels(t *testing.T) {
	t.Parallel()

	s := subtitleOf("JOHN: Hi.", "DR. O'NEIL: Yes.\nMARY: No.")
	_, err := speakers.Apply(s, speakers.Options{FromLabels: true, Style: speakers.StylePrefix})
	require.NoError(t, err)

	got := make([][]string, 0, s.Len())
	for _, v := range viewsOf(s) {
		got = append(got, v.Lines)
	}
	assert.Equal(t, [][]string{{"JOHN: Hi."}, {"DR. O'NEIL: Yes.", "MARY: No."}}, got)
}

// Adapted from SpeakerLabelsTest::testCuesWithoutSpeakersStayUnchanged.
func TestCuesWithoutSpeakersStayUnchanged(t *testing.T) {
	t.Parallel()

	for _, style := range []speakers.Style{speakers.StyleNone, speakers.StylePrefix, speakers.StyleDialogueDashes, speakers.StyleColors} {
		t.Run(style.String(), func(t *testing.T) {
			t.Parallel()

			s := subtitleOf("<i>Hi</i> there")
			before := viewsOf(s)
			report, err := speakers.Apply(s, speakers.Options{Style: style})
			require.NoError(t, err)
			assert.Zero(t, report.ChangedCues)
			assert.Equal(t, before, viewsOf(s))
		})
	}
}

func TestZeroOptionsChangeNothing(t *testing.T) {
	t.Parallel()

	s := load(t, "speakers/voices.vtt")
	report, err := speakers.Apply(s, speakers.Options{})
	require.NoError(t, err)
	assert.Zero(t, report.ChangedCues)
	assert.Equal(t, viewsOf(load(t, "speakers/voices.vtt")), viewsOf(s))
}

// Adapted from SpeakerLabelsTest::testListCountsTheCuesOfEachSpeakerInOrderOfAppearance and testListKeepsTheDecodedNames.
func TestList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   *model.Subtitle
		want []speakers.SpeakerCount
	}{
		{"voices.vtt", load(t, "speakers/voices.vtt"), []speakers.SpeakerCount{{"Anna", 3}, {"Ben", 3}, {"Clara", 2}}},
		{"no voices", subtitleOf("No speaker here."), nil},
		{"same speaker twice in a cue", subtitleOf("<v Ben>Hi.\n<v Ben>Again."), []speakers.SpeakerCount{{"Ben", 1}}},
		{"decoded names", subtitleOf("<v Tom &amp; Jerry>Hi. <v O&#39;Neil>Hello."), []speakers.SpeakerCount{{"Tom & Jerry", 1}, {"O'Neil", 1}}},
		{"numeric names stay strings", subtitleOf("<v 0>Hi.", "<v 1>Yes.", "<v 0>Bye."), []speakers.SpeakerCount{{"0", 2}, {"1", 1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, speakers.List(tt.in))
		})
	}
}

// Adapted from SpeakerLabelsTest::testVoicesSurviveWebVttAndTtml.
func TestVoicesSurviveWebVTTAndTTML(t *testing.T) {
	t.Parallel()

	for _, f := range []stbox.Format{stbox.WebVTT, stbox.TTML} {
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			s := subtitleOf("DR. O'NEIL: Yes.\nMARY: No.")
			_, err := speakers.Apply(s, speakers.Options{FromLabels: true})
			require.NoError(t, err)
			var buf bytes.Buffer
			_, err = stbox.Write(&buf, s, f, stbox.WriteOptions{})
			require.NoError(t, err)

			back, _, err := stbox.Parse(bytes.NewReader(buf.Bytes()), int64(buf.Len()), stbox.ReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, []speakers.SpeakerCount{{"Dr. O'Neil", 1}, {"Mary", 1}}, speakers.List(back))
		})
	}
}

// Adapted from SpeakerLabelsTest::testOptionsRejectInvalidColours.
func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts speakers.Options
		want string
	}{
		{"zero", speakers.Options{}, ""},
		{"valid colors", speakers.Options{Colors: []string{"#FFFF00", "#00ff00"}}, ""},
		{"color name", speakers.Options{Colors: []string{"yellow"}}, `stbox: invalid argument: speakers.Options.Colors: The speaker colors must be a list of colors such as "#ffff00", got "yellow".`},
		{"short color", speakers.Options{Colors: []string{"#ff0"}}, `stbox: invalid argument: speakers.Options.Colors: The speaker colors must be a list of colors such as "#ffff00", got "#ff0".`},
		{"empty color", speakers.Options{Colors: []string{"#ffff00", ""}}, `stbox: invalid argument: speakers.Options.Colors: The speaker colors must be a list of colors such as "#ffff00", got "".`},
		{"dash style", speakers.Options{DialogueDashStyle: model.DialogueDashStyle(8)}, "stbox: invalid argument: speakers.Options.DialogueDashStyle: The dialogue dash style must be one of the 8 constants, got DialogueDashStyle(8)."},
		{"style", speakers.Options{Style: speakers.Style(4)}, "stbox: invalid argument: speakers.Options.Style: The speaker style must be one of the Style constants, got Style(4)."},
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
			require.EqualError(t, err, tt.want)

			s := subtitleOf("<v Anna>Hi.")
			_, err = speakers.Apply(s, tt.opts)
			require.Error(t, err)
			c, _ := s.Cue(0)
			assert.Equal(t, "Anna", c.Speaker)
		})
	}
}
