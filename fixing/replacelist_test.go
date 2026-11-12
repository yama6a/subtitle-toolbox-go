package fixing_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/fixing"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func rep(pairs ...string) []fixing.Replacement {
	var out []fixing.Replacement
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, fixing.Replacement{From: pairs[i], To: pairs[i+1]})
	}
	return out
}

func readList(t *testing.T, xml string) *fixing.ReplaceList {
	t.Helper()

	l, err := fixing.ReadReplaceList(strings.NewReader(xml))
	require.NoError(t, err)
	return l
}

func TestReadReplaceListReadsTheSectionsOfAUserFile(t *testing.T) {
	t.Parallel()

	l, err := fixing.ReadReplaceList(strings.NewReader(string(testfiles.Read(t, "fixing/user_OCRFixReplaceList.xml"))))
	require.NoError(t, err)
	assert.Equal(t, &fixing.ReplaceList{
		WholeWords:         rep("celd", "cold", "oD", "on", "trom", "from"),
		PartialWordsAlways: rep("iD", "in"),
		WholeLines:         rep("Ple*se kee* you* *icket.", "Please keep your ticket."),
		BeginLines:         rep("T*e ", "The "),
		EndLines:           rep(" deck'", " deck."),
		PartialLines:       rep("*'*.", "p.m."),
		PartialLinesAlways: rep("Pe* hou*", "per hour"),
		RegularExpressions: rep(`(\p{L})'$`, "$1.", `\b(\d+) k\*`, "$1 km", "x", "${name}"),
	}, l)
}

func TestReadReplaceListSkipsEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, xml string
		want      fixing.ReplaceList
	}{
		{
			"first entry wins, also across sections of one name, and entries without change go",
			"<ReplaceList><WholeWords><Word from='Tne' to='The' /><Word from='Tne' to='Ten' /><Word from='keep' to='keep' />" +
				"<Word from='' to='x' /><Word to='y' /><Word from='z' /><!-- note --></WholeWords>" +
				"<WholeWords><Word from='lt' to='It' /><Word from='Tne' to='Tee' /></WholeWords></ReplaceList>",
			fixing.ReplaceList{WholeWords: rep("Tne", "The", "lt", "It")},
		},
		{
			"sections that need a spell checker or edit the shipped list",
			"<OCRFixReplaceList><RemovedWholeWords><Word from='lt' to='' /></RemovedWholeWords>" +
				"<PartialWords><WordPart from='1' to='l' /></PartialWords><Other><X from='a' to='b' /></Other>" +
				"<RegularExpressionsIfSpelledCorrectly><RegEx find='a' replaceWith='b' /></RegularExpressionsIfSpelledCorrectly></OCRFixReplaceList>",
			fixing.ReplaceList{},
		},
		{
			"regular expressions without replaceWith, with an empty find or that do not compile",
			"<L><RegularExpressions><RegEx find='d' /><RegEx find='' replaceWith='e' /><RegEx find='(?&lt;=a)(' replaceWith='b' />" +
				"<RegEx find='(a)/b' replaceWith='$$ $&amp; \\ ${1}' /><RegEx find='(a)/b' replaceWith='second' /></RegularExpressions></L>",
			fixing.ReplaceList{RegularExpressions: rep("(a)/b", `$$ $& \ ${1}`)},
		},
		{
			"byte order mark",
			"\ufeff<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<L><WholeWords><Word from='a' to='b' /></WholeWords></L>",
			fixing.ReplaceList{WholeWords: rep("a", "b")},
		},
		{
			"encoding declaration",
			"<?xml version=\"1.0\" encoding=\"iso-8859-1\"?><L><WholeLines><Line from='caf\xe9' to='cafe' /></WholeLines></L>",
			fixing.ReplaceList{WholeLines: rep("café", "cafe")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, &tt.want, readList(t, tt.xml))
		})
	}
}

func TestReadReplaceListFailsForInvalidXML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, xml string
		line      int
	}{
		{"closing tag of another element", "<ReplaceList>\n<WholeWords></ReplaceList>", 2},
		{"empty input", "", 1},
		{"second root element", "<L><WholeWords><Word from='a' to='b' /></WholeWords></L><L></L>", 1},
		{"text after the root", "<L></L>text", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := fixing.ReadReplaceList(strings.NewReader(tt.xml))
			require.ErrorIs(t, err, model.ErrParse)
			pe, ok := errors.AsType[*model.ParseError](err)
			require.True(t, ok)
			assert.Empty(t, pe.Format)
			assert.Equal(t, tt.line, pe.Line)
			assert.True(t, strings.HasPrefix(pe.Msg, "The OCR replace list is not valid XML: "), pe.Msg)
		})
	}
}

func TestReplaceListValidate(t *testing.T) {
	t.Parallel()

	opts := fixing.Options{ReplaceList: &fixing.ReplaceList{RegularExpressions: rep("[", "")}}
	err := opts.Validate()
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	assert.Contains(t, err.Error(), `fixing.Options.ReplaceList: The regular expression "[" is not valid: `)

	opts = fixing.Options{ReplaceList: &fixing.ReplaceList{RegularExpressions: rep("", "-")}}
	require.ErrorIs(t, opts.Validate(), model.ErrInvalidArgument)

	err = fixing.Options{Enable: []fixing.Rule{fixing.RuleReplaceList}}.Validate()
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "The rule replaceList needs a replace list.")
}

func TestReplaceListRuleAppliesEachSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		list         fixing.ReplaceList
		before, want []string
	}{
		{"whole word with punctuation", fixing.ReplaceList{WholeWords: rep("Tne", "The")}, []string{"\"Tne. Tne, Tneran"}, []string{"\"The. The, Tneran"}},
		{"whole word with its comma", fixing.ReplaceList{WholeWords: rep("l,m", "I'm")}, []string{"l,m in."}, []string{"I'm in."}},
		{
			"partial word, then whole word",
			fixing.ReplaceList{WholeWords: rep("will", "will"), PartialWordsAlways: rep("II", "ll")},
			[]string{"wiII it"},
			[]string{"will it"},
		},
		{"whole line", fixing.ReplaceList{WholeLines: rep("H ey.", "Hey.")}, []string{"H ey.", "H ey. Hi"}, []string{"Hey.", "H ey. Hi"}},
		{
			"begin of a line and sentence",
			fixing.ReplaceList{BeginLines: rep("lgot ", "I got ")},
			[]string{"- lgot it. lgot it"},
			[]string{"- I got it. I got it"},
		},
		{"end of the cue", fixing.ReplaceList{EndLines: rep(" sin", " sir.")}, []string{"Yes, sin", "Yes, sin"}, []string{"Yes, sin", "Yes, sir."}},
		{
			"between word boundaries",
			fixing.ReplaceList{PartialLines: rep("aren '1'", "aren't")},
			[]string{"We aren '1' late, aren '1'x"},
			[]string{"We aren't late, aren '1'x"},
		},
		{"always in a line", fixing.ReplaceList{PartialLinesAlways: rep("Apollo 1 3", "Apollo 13")}, []string{"Apollo 1 3!"}, []string{"Apollo 13!"}},
		{
			"regular expression",
			fixing.ReplaceList{RegularExpressions: rep(`\b1 (know|will)\b`, "I $1")},
			[]string{"1 know 1 will"},
			[]string{"I know I will"},
		},
		{
			"whole word with the punctuation around it",
			fixing.ReplaceList{WholeWords: rep("celd", "cold", "-oD.", "-on.")},
			[]string{"\"(celd),\" -oD. (celd!)"},
			[]string{"\"(cold),\" -on. (cold!)"},
		},
		{
			"partial line next to a music note and a letter with an accent",
			fixing.ReplaceList{PartialLines: rep("*'*.", "p.m.")},
			[]string{"At 5 *'*.♪ and é*'*."},
			[]string{"At 5 p.m.♪ and é*'*."},
		},
		{
			".NET substitutions",
			fixing.ReplaceList{RegularExpressions: rep("(a)/b", `$$ $& \ ${1}`, "c", "${name}")},
			[]string{"a/b x c"},
			[]string{`$ a/b \ a x ${name}`},
		},
		{
			"anchors at tags",
			fixing.ReplaceList{RegularExpressions: rep("^x", "y")},
			[]string{"x<i>x</i>"},
			[]string{"y<i>y</i>"},
		},
		{"text that is not valid UTF-8 stays", fixing.ReplaceList{RegularExpressions: rep("a", "b")}, []string{"a\xe9"}, []string{"a\xe9"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := only(fixing.RuleReplaceList)
			opts.ReplaceList = &tt.list
			got, _ := fixLines(t, tt.before, opts)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplaceListEndLinesLookAtTheNextCue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		gap   time.Duration
		next  string
		endTo string
		want  string
	}{
		{"next cue continues at 600 ms", 600 * time.Millisecond, "<i>and</i> she said", " mother.", "I asked mothen"},
		{"gap of 601 ms", 601 * time.Millisecond, "and she said", " mother.", "I asked mother."},
		{"next cue starts in upper case", 100 * time.Millisecond, "And she said", " mother.", "I asked mother."},
		{"replacement without a full stop", 100 * time.Millisecond, "and she said", " mother", "I asked mother"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var s model.Subtitle
			first := model.Cue{Start: time.Second, End: 2 * time.Second}
			first.SetText("I asked mothen")
			next := model.Cue{Start: 2*time.Second + tt.gap, End: 5 * time.Second}
			next.SetText(tt.next)
			last := model.Cue{Start: 6 * time.Second, End: 7 * time.Second}
			last.SetText("Ask mothen")
			s.AddCue(first)
			s.AddCue(next)
			s.AddCue(last)

			opts := only(fixing.RuleReplaceList)
			opts.ReplaceList = &fixing.ReplaceList{EndLines: rep(" mothen", tt.endTo)}
			_, err := fixing.Apply(&s, opts)
			require.NoError(t, err)
			assert.Equal(t, []string{tt.want, tt.next, "Ask" + tt.endTo}, texts(&s))
		})
	}
}

func TestReplaceListRemovesACueThatItEmpties(t *testing.T) {
	t.Parallel()

	s := fromTexts("Keep", "<i>Subtitles by Jane</i>", "lt's me")
	report, err := fixing.Apply(s, fixing.Options{
		Language: "en", ReplaceList: &fixing.ReplaceList{WholeLines: rep("Subtitles by Jane", "")},
	})
	require.NoError(t, err)

	assert.Equal(t, []fixing.Fix{
		{CueIndex: 1, Rule: fixing.RuleReplaceList, Before: "<i>Subtitles by Jane</i>", After: "<i></i>"},
		{CueIndex: 1, Rule: fixing.RuleEmptyTags, Before: "<i></i>"},
		{CueIndex: 2, Rule: fixing.RuleOCRLowercaseL, Before: "lt's me", After: "It's me"},
	}, report.Fixes)
	assert.Equal(t, []string{"Keep", "It's me"}, texts(s))
}

func TestReplaceListTimeoutLeavesTheSubtitleUnchanged(t *testing.T) {
	t.Parallel()

	slow := strings.Repeat("a", 40) + "b"
	s := fromTexts("lt's", slow+"<i>"+slow+"</i>"+slow+"<b>"+slow+"</b>")
	start := time.Now()
	opts := fixing.Options{Language: "en", ReplaceList: &fixing.ReplaceList{RegularExpressions: rep(`^(a+)+$`, "")}}
	_, err := fixing.Apply(s, opts)
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	assert.Contains(t, err.Error(), `The regular expression "^(a+)+$" took longer than 1 s on cue 2.`)
	assert.Less(t, time.Since(start), 3500*time.Millisecond)
	assert.Equal(t, []string{"lt's", slow + "<i>" + slow + "</i>" + slow + "<b>" + slow + "</b>"}, texts(s))
}

func TestReplaceListKeepsTheImageOfAnImageCue(t *testing.T) {
	t.Parallel()

	image := &model.Image{PNG: []byte{1, 2, 3}, Width: 10, Height: 10, ScreenWidth: 720, ScreenHeight: 576}
	var s model.Subtitle
	s.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second, Image: image})
	s.AddCue(model.Cue{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"lt gets celd"}, Image: image})
	report, err := fixing.Apply(&s, fixing.Options{
		Language: "en", ReplaceList: &fixing.ReplaceList{WholeWords: rep("celd", "cold")},
	})
	require.NoError(t, err)

	require.Equal(t, 2, s.Len())
	assert.Equal(t, []fixing.Rule{fixing.RuleReplaceList, fixing.RuleOCRLowercaseL}, rules(report.Fixes))
	for i, want := range [][]string{nil, {"It gets cold"}} {
		c, _ := s.Cue(i)
		assert.Equal(t, image, c.Image)
		assert.Equal(t, want, c.Lines)
	}
}
