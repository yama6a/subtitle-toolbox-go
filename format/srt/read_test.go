package srt_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports the exception tests of tests/Parsers/SubRipParserTest.php of subtitle-toolbox-php.
func TestReadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantMsg  string
		wantLine int
	}{
		{
			name:     "4 hour digits",
			content:  string(testfiles.Read(t, "srt/exceeded_hours.srt")),
			wantMsg:  `The time "1234:00:00,456" is not valid.`,
			wantLine: 2,
		},
		{
			name:     "minutes of 99",
			content:  string(testfiles.Read(t, "srt/exceeded_minutes.srt")),
			wantMsg:  `The time "55:99:00,456" is not valid.`,
			wantLine: 2,
		},
		{
			name:     "seconds of 99",
			content:  string(testfiles.Read(t, "srt/exceeded_seconds.srt")),
			wantMsg:  `The time "34:00:99,123" is not valid.`,
			wantLine: 2,
		},
		{
			name:     "4 fraction digits",
			content:  string(testfiles.Read(t, "srt/exceeded_milli_accuracy.srt")),
			wantMsg:  `The time "34:00:00,1234" is not valid.`,
			wantLine: 2,
		},
		{
			name:     "missing cue number",
			content:  string(testfiles.Read(t, "srt/missing_cue_number.srt")),
			wantMsg:  `Block #3 has no cue number on its first line. The line is "01:01:01,000 --> 80:00:00,800".`,
			wantLine: 13,
		},
		{
			name:     "missing timing line",
			content:  string(testfiles.Read(t, "srt/missing_timestamps.srt")),
			wantMsg:  `Block #2 has no timing line on its second line. The line is "Minutes and seconds and milliseconds".`,
			wantLine: 10,
		},
		{
			name:     "missing text and a time of 999 hours with minutes of 99",
			content:  string(testfiles.Read(t, "srt/missing_text.srt")),
			wantMsg:  `The time "999:99:99,999" is not valid.`,
			wantLine: 17,
		},
		{
			name:     "a block with only a cue number",
			content:  "1\n00:00:01,000 --> 00:00:02,000\nFirst\n\n2",
			wantMsg:  "Block #1 has no timing line on its second line.",
			wantLine: 5,
		},
		{
			name:     "incomplete coordinates",
			content:  "1\n00:00:01,000 --> 00:00:04,000 X1:100 X2:600\nText\n",
			wantMsg:  `The text "X1:100 X2:600" after the end time is not valid.`,
			wantLine: 2,
		},
		{
			name:     "a separator without a fraction",
			content:  "1\n00:01:39, --> 00:01:41,000\nText\n",
			wantMsg:  `The time "00:01:39," is not valid.`,
			wantLine: 2,
		},
		{
			name:     "unknown text after cue settings",
			content:  "1\n00:00:01,000 --> 00:00:03,500 align:start region:top\nText\n",
			wantMsg:  `The text "region:top" after the end time is not valid.`,
			wantLine: 2,
		},
		{
			name:     "a cue that ends before it starts",
			content:  "1\n00:00:03,000 --> 00:00:01,500\nText\n",
			wantMsg:  "The cue ends at 1.5 s, before it starts at 3 s.",
			wantLine: 2,
		},
		{
			name:     "a long cue number line is cut in the quote",
			content:  "Downloaded from a site with a very long name that goes on and on\n00:00:01,000 --> 00:00:02,000\n",
			wantMsg:  `Block #0 has no cue number on its first line. The line is "Downloaded from a site with a very long name that goes on...".`,
			wantLine: 1,
		},
		{
			name:     "a block of full-width digits and a colon after a cue stays apart",
			content:  "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n\uff11\uff12\uff1a\uff13\uff14 text\n",
			wantMsg:  "Block #1 has no cue number on its first line. The line is \"\uff11\uff12\uff1a\uff13\uff14 text\".",
			wantLine: 5,
		},
		{
			name:     "a block of digits, an ideographic space and a colon after a cue stays apart",
			content:  "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n12\u3000:34 x\n",
			wantMsg:  "Block #1 has no cue number on its first line. The line is \"12\u3000:34 x\".",
			wantLine: 5,
		},
		{
			name:     "an empty end time",
			content:  "1\n00:00:01,000 -->\nText\n",
			wantMsg:  `The time "" is not valid.`,
			wantLine: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perr := readError(t, tt.content)
			assert.Equal(t, tt.wantMsg, perr.Msg)
			assert.Equal(t, tt.wantLine, perr.Line)
		})
	}
}

// Ports the cue tests of tests/Parsers/SubRipParserTest.php of subtitle-toolbox-php.
func TestReadCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []cueRow
	}{
		{
			name:    "a cue without text has no lines",
			content: "1\n00:00:01,000 --> 00:00:02,000",
			want:    []cueRow{{time.Second, 2 * time.Second, nil}},
		},
		{
			name:    "several empty lines between cues",
			content: "1\n00:00:01,000 --> 00:00:02,000\nFirst\n\n \n\n2\n00:00:03,000 --> 00:00:04,000\nSecond\n",
			want:    []cueRow{{time.Second, 2 * time.Second, []string{"First"}}, {3 * time.Second, 4 * time.Second, []string{"Second"}}},
		},
		{
			name:    "a dot, 1 hour digit and short fractions",
			content: "1\n0:00:01.5 --> 00:00:02,25\nText\n\n2\n00:00:03.000 --> 01:02:03.004\nDots\n",
			want:    []cueRow{{1500 * ms, 2250 * ms, []string{"Text"}}, {3 * time.Second, 3723004 * ms, []string{"Dots"}}},
		},
		{
			name:    "times without fraction",
			content: "1\n00:01:39 --> 00:01:41,000\nText\n\n2\n00:01:42,500 --> 00:01:44\nMore\n",
			want:    []cueRow{{99 * time.Second, 101 * time.Second, []string{"Text"}}, {102500 * ms, 104 * time.Second, []string{"More"}}},
		},
		{
			name:    "a colon and 3 digits before the fraction",
			content: "1\n00:00:01:105 --> 00:00:02:200\nText\n",
			want:    []cueRow{{1105 * ms, 2200 * ms, []string{"Text"}}},
		},
		{
			name:    "a run of CRs before LF is one line end",
			content: "1\r\r\n00:00:01,000 --> 00:00:02,000\r\r\nFirst\r\r\nline\r\r\n\r\r\n2\r\r\n00:00:03,000 --> 00:00:04,000\r\r\nSecond\r\r\n",
			want:    []cueRow{{time.Second, 2 * time.Second, []string{"First", "line"}}, {3 * time.Second, 4 * time.Second, []string{"Second"}}},
		},
		{
			name:    "a lone CR ends a line",
			content: "1\r00:00:01,000 --> 00:00:02,000\rFirst\r\r2\r00:00:03,000 --> 00:00:04,000\rSecond",
			want:    []cueRow{{time.Second, 2 * time.Second, []string{"First"}}, {3 * time.Second, 4 * time.Second, []string{"Second"}}},
		},
		{
			name:    "the arrow without spaces",
			content: "1\n00:00:01,000-->00:00:02,000\nText\n",
			want:    []cueRow{{time.Second, 2 * time.Second, []string{"Text"}}},
		},
		{
			name:    "tabs around the arrow",
			content: "1\n00:00:01,000\t-->\t\t00:00:02,000\nText\n",
			want:    []cueRow{{time.Second, 2 * time.Second, []string{"Text"}}},
		},
		{
			name:    "cue numbers in PHP numeric forms",
			content: "+1\n00:00:01,000 --> 00:00:02,000\nA\n\n1.5\n00:00:03,000 --> 00:00:04,000\nB\n\n1e3\n00:00:05,000 --> 00:00:06,000\nC\n",
			want: []cueRow{
				{time.Second, 2 * time.Second, []string{"A"}},
				{3 * time.Second, 4 * time.Second, []string{"B"}},
				{5 * time.Second, 6 * time.Second, []string{"C"}},
			},
		},
		{
			name:    "cues in another order are sorted",
			content: "1\n00:00:05,000 --> 00:00:06,000\nLater\n\n2\n00:00:01,000 --> 00:00:02,000\nEarlier\n",
			want:    []cueRow{{time.Second, 2 * time.Second, []string{"Earlier"}}, {5 * time.Second, 6 * time.Second, []string{"Later"}}},
		},
		{
			name:    "BOMs of joined files and runs of spaces",
			content: "\ufeff1\n00:00:01,000 --> 00:00:02,000\nOne  \t two\n\n\ufeff\ufeff2\n00:00:03,000 --> 00:00:04,000\nThree\n",
			want:    []cueRow{{time.Second, 2 * time.Second, []string{"One two"}}, {3 * time.Second, 4 * time.Second, []string{"Three"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, false)
			assert.Equal(t, tt.want, cueRows(s))
			assert.Empty(t, warnings)
		})
	}
}

// Ports testAFileWithoutCuesReadsAsZeroCues of tests/Parsers/SubRipParserTest.php of subtitle-toolbox-php.
func TestReadWithoutCues(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", "\ufeff", " \n\t\r\n\n  ", "\ufeff\r\n\r\n"} {
		for _, lenient := range []bool{false, true} {
			s, warnings := mustRead(t, content, lenient)
			assert.Zero(t, s.Len())
			assert.Empty(t, warnings)
		}
	}
}

// Rows of the time table of https://github.com/yama6a/subtitle-toolbox-go/issues/20.
func TestReadTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		start      string
		want       time.Duration
		strictOK   bool
		lenientOK  bool
		looseValue string
	}{
		{name: "standard", start: "00:00:01,500", want: 1500 * ms, strictOK: true, lenientOK: true},
		{name: "dot", start: "00:00:01.500", want: 1500 * ms, strictOK: true, lenientOK: true},
		{name: "3 hour digits", start: "999:59:59,999", want: 999*time.Hour + 59*time.Minute + 59999*ms, strictOK: true, lenientOK: true},
		{name: "1 fraction digit", start: "00:00:01,5", want: 1500 * ms, strictOK: true, lenientOK: true},
		{name: "2 fraction digits", start: "00:00:01,05", want: 1050 * ms, strictOK: true, lenientOK: true},
		{name: "no fraction", start: "00:00:01", want: time.Second, strictOK: true, lenientOK: true},
		{name: "colon and 3 digits", start: "00:00:01:500", want: 1500 * ms, strictOK: true, lenientOK: true},
		{name: "1-digit fields", start: "0:0:1", lenientOK: false},
		{name: "1-digit fields with a fraction", start: "0:0:1,500", want: 1500 * ms, lenientOK: true, looseValue: "1.5"},
		{name: "4 fraction digits", start: "00:00:01,5000", want: 1500 * ms, lenientOK: true, looseValue: "1.5"},
		{name: "4 fraction digits round half up", start: "00:00:01,0005", want: 1001 * ms, lenientOK: true, looseValue: "1.001"},
		{name: "colon and 2 digits", start: "00:00:01:50", want: 1500 * ms, lenientOK: true, looseValue: "1.5"},
		{name: "no hours", start: "07:03,920", want: 423920 * ms, lenientOK: true, looseValue: "423.92"},
		{name: "leading zeros on 5 hour digits", start: "0099999:00:00,000", want: 99999 * time.Hour, lenientOK: true, looseValue: "359996400"},
		{name: "6 hour digits", start: "100000:00:00,000"},
		{name: "cut off last field", start: "00:00:0"},
		{name: "minutes of 60", start: "00:60:00,000"},
		{name: "seconds of 60", start: "00:00:60,000"},
		{name: "letters", start: "00:00:0G,000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := "1\n" + tt.start + " --> " + tt.start + "\nThe ferry leaves.\n"
			invalid := `The time "` + tt.start + `" is not valid.`
			if tt.strictOK {
				s, _ := mustRead(t, content, false)
				assert.Equal(t, tt.want, firstCue(t, s).Start)
			} else {
				assert.Equal(t, invalid, readError(t, content).Msg)
			}

			s, warnings := mustRead(t, content, true)
			switch {
			case !tt.lenientOK:
				assert.Zero(t, s.Len())
				require.Len(t, warnings, 1)
				assert.Equal(t, warningRow{invalid, 2, 0, model.Skipped}, warningRows(warnings)[0])
			case tt.looseValue != "":
				assert.Equal(t, tt.want, firstCue(t, s).Start)
				msg := `Block #0 has the time "` + tt.start + `", which is not in the form hh:mm:ss,mmm. The parser read it as ` + tt.looseValue + " s."
				assert.Equal(t, []warningRow{{msg, 2, 0, model.Repaired}}, warningRows(warnings))
			default:
				assert.Equal(t, tt.want, firstCue(t, s).Start)
				assert.Empty(t, warnings)
			}
		})
	}
}

// Ports the loose time cases of tests/Parsers/LooseTimestampsTest.php of subtitle-toolbox-php.
func TestReadLooseTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		start, end string
		want       cueRow
		strict     bool
	}{
		{"colon before the fraction", "00:00:01:105", "00:00:02:200", cueRow{1105 * ms, 2200 * ms, nil}, true},
		{"4 fraction digits", "00:28:41,1000", "00:28:42,0006", cueRow{1721100 * ms, 1722001 * ms, nil}, false},
		{"1-digit minutes", "0:0:1,500", "0:0:2,500", cueRow{1500 * ms, 2500 * ms, nil}, false},
		{"1-digit seconds", "00:00:0,000", "00:00:2,000", cueRow{0, 2 * time.Second, nil}, false},
		{"without hours", "07:03,920", "07:04,500", cueRow{423920 * ms, 424500 * ms, nil}, false},
		{"without fraction", "00:00:04", "00:00:05", cueRow{4 * time.Second, 5 * time.Second, nil}, true},
		{"4 hour digits", "0000:00:01,000", "0000:00:02,000", cueRow{time.Second, 2 * time.Second, nil}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := "1\n" + tt.start + " --> " + tt.end + "\nThe ferry leaves.\n\n2\n02:00:00,000 --> 02:00:02,000\nIt is late.\n"
			tt.want.Lines = []string{"The ferry leaves."}
			s, warnings := mustRead(t, content, true)
			assert.Equal(t, []cueRow{tt.want, {2 * time.Hour, 2*time.Hour + 2*time.Second, []string{"It is late."}}}, cueRows(s))
			if tt.strict {
				assert.Empty(t, warnings)
			} else {
				require.Len(t, warnings, 2)
				for _, w := range warnings {
					assert.Equal(t, model.Repaired, w.Action)
					assert.Equal(t, 2, w.Line)
				}
			}

			if !tt.strict {
				assert.Contains(t, readError(t, content).Msg, "is not valid.")
				return
			}
			s, _ = mustRead(t, content, false)
			assert.Equal(t, tt.want, cueRows(s)[0])
		})
	}
}

// Ports testLenientModeSkipsACueWithMinutesOrSecondsOfSixtyOrMore and the cut-off and same time cases of
// tests/Parsers/LooseTimestampsTest.php of subtitle-toolbox-php.
func TestReadLenientSkipsBadTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, timing string
	}{
		{"minutes", "00:00:01,000 --> 00:75:02,000"},
		{"seconds", "00:00:01,000 --> 00:00:75,000"},
		{"without hours", "00:01,000 --> 00:60,000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := "1\n" + tt.timing + "\nThe ferry leaves.\n\n2\n02:00:00,000 --> 02:00:02,000\nIt is late.\n"
			s, warnings := mustRead(t, content, true)
			assert.Equal(t, []cueRow{{2 * time.Hour, 2*time.Hour + 2*time.Second, []string{"It is late."}}}, cueRows(s))
			require.Len(t, warnings, 1)
			assert.Equal(t, model.Skipped, warnings[0].Action)
			_ = readError(t, content)
		})
	}

	t.Run("a time that the end of the file cut off", func(t *testing.T) {
		t.Parallel()

		s, warnings := mustRead(t, "1\n00:00:01,000 --> 00:00:0", true)
		assert.Zero(t, s.Len())
		assert.Equal(t, []warningRow{{`The time "00:00:0" is not valid.`, 2, 0, model.Skipped}}, warningRows(warnings))
	})
	t.Run("the same loose time twice warns once", func(t *testing.T) {
		t.Parallel()

		_, warnings := mustRead(t, "1\n0:0:1,5 --> 0:0:1,5\nText\n", true)
		assert.Equal(t, []warningRow{{
			`Block #0 has the time "0:0:1,5", which is not in the form hh:mm:ss,mmm. The parser read it as 1.5 s.`, 2, 0, model.Repaired,
		}}, warningRows(warnings))
	})
}

// Ports the escaping and tag tests of tests/Parsers/SubRipParserTest.php of subtitle-toolbox-php.
func TestReadText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		want  []string
		align model.Alignment
	}{
		{
			name: "text outside tags is escaped",
			text: "I <3 bread & jam\n<i>Salt & pepper</i> 2 > 1",
			want: []string{"I &lt;3 bread &amp; jam", "<i>Salt &amp; pepper</i> 2 &gt; 1"},
		},
		{
			name: "an entity in the file is text",
			text: "The sign says &amp; and &lt;b&gt;",
			want: []string{"The sign says &amp;amp; and &amp;lt;b&amp;gt;"},
		},
		{name: "an entity without its semicolon", text: "AT&amp T", want: []string{"AT&amp;amp T"}},
		{
			name: "core tags, attribute tags and unknown pairs stay markup",
			text: `<B>bold</B> <font color="#00aa00">green</font> <foo>unknown</foo>`,
			want: []string{`<B>bold</B> <font color="#00aa00">green</font> <foo>unknown</foo>`},
		},
		{
			name: "an unknown tag without attributes and end tag is text",
			text: "<b>Rye</b> & <b>spelt</b> <today>\n<i>open<br><foo>a\nb</FOO> <span class=\"x\">",
			want: []string{"<b>Rye</b> &amp; <b>spelt</b> &lt;today&gt;", "<i>open<br><foo>a", `b</FOO> <span class="x">`},
		},
		{
			name: "text in angle brackets",
			text: `<a sentence in brackets> <span class="x">Exit</span> <to the left>`,
			want: []string{`&lt;a sentence in brackets&gt; <span class="x">Exit</span> &lt;to the left&gt;`},
		},
		{name: "a slash right after the name is no tag", text: "a<b/>b", want: []string{"a&lt;b/&gt;b"}},
		{name: "a self-closing tag", text: "a<br />b", want: []string{"a<br />b"}},
		{name: "an attribute with single quotes", text: "<span title='x y'>t</span>", want: []string{"<span title='x y'>t</span>"}},
		{
			name:  "the alignment tag goes to the cue",
			text:  `{\an8}<i>The train leaves soon</i>`,
			want:  []string{"<i>The train leaves soon</i>"},
			align: model.TopCenter,
		},
		{
			name:  "the first alignment tag wins and all go",
			text:  `{\an4}Middle and horiz{\an6}ontally left`,
			want:  []string{"Middle and horizontally left"},
			align: model.MiddleLeft,
		},
		{name: "a legacy code without numpad value stays", text: `{\a4}Text`, want: []string{`{\a4}Text`}},
		{name: "a legacy code", text: `{\a6}Text`, want: []string{"Text"}, align: model.TopCenter},
		{
			name: "style toggles become core tags",
			text: `{\b1}bold{\b0} {\i1}italic{\i0} {\u1}under{\u0} {\s1}struck{\s0}`,
			want: []string{"<b>bold</b> <i>italic</i> <u>under</u> <s>struck</s>"},
		},
		{
			name:  "open toggles close at the end of the cue in reverse order",
			text:  "{\\an8\\i1}one {\\b1}two\nthree{\\u0}",
			want:  []string{"<i>one <b>two", "three</b></i>"},
			align: model.TopCenter,
		},
		{name: "a toggle across lines", text: "{\\i1}one\ntwo{\\i0} three", want: []string{"<i>one", "two</i> three"}},
		{name: "an unclosed italic toggle", text: `{\i1}one`, want: []string{"<i>one</i>"}},
		{name: "a toggle to the current state writes nothing", text: `{\b1}a{\b1}b{\b0}{\b0}`, want: []string{"<b>ab</b>"}},
		{name: "a closing toggle closes at once", text: `{\b1}a{\i1}b{\b0}c`, want: []string{"<b>a<i>b</b>c</i>"}},
		{
			name:  "unknown override tags stay in 1 brace block",
			text:  `{\an8\fad(200,200)}Sign {\pos(10,20)}here {normal text}`,
			want:  []string{`{\fad(200,200)}Sign {\pos(10,20)}here {normal text}`},
			align: model.TopCenter,
		},
		{name: "converted tags come before the unknown tags", text: `{\b1\pos(1,2)}bold`, want: []string{`<b>{\pos(1,2)}bold</b>`}},
		{name: "a bold weight is no toggle", text: `{\b700}bold`, want: []string{`{\b700}bold`}},
		{name: "a line with only an alignment tag drops out", text: "{\\an8}\nText", want: []string{"Text"}, align: model.TopCenter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, "1\n00:00:01,000 --> 00:00:04,000\n"+tt.text+"\n", false)
			c := firstCue(t, s)
			assert.Equal(t, tt.want, c.Lines)
			assert.Equal(t, tt.align, c.Alignment)
		})
	}
}

// Ports the coordinates and cue settings tests of tests/Parsers/SubRipParserTest.php of subtitle-toolbox-php.
func TestReadTextAfterEndTime(t *testing.T) {
	t.Parallel()

	coordinates := func(x1, x2, y1, y2 int64) *model.Data {
		return model.NewData(model.Field{Key: "coordinates", Value: model.NewData(
			model.Field{Key: "x1", Value: x1}, model.Field{Key: "x2", Value: x2},
			model.Field{Key: "y1", Value: y1}, model.Field{Key: "y2", Value: y2},
		)})
	}
	settings := func(pairs ...[2]string) *model.Data {
		d := model.NewData()
		for _, p := range pairs {
			d.Set(p[0], p[1])
		}
		return d
	}
	tests := []struct {
		name    string
		rest    string
		text    string
		wantSRT *model.Data
		wantVTT *model.Data
	}{
		{name: "coordinates", rest: " X1:100 X2:600 Y1:40 Y2:80", text: "Text", wantSRT: coordinates(100, 600, 40, 80)},
		{name: "coordinates with leading zeros", rest: " X1:0 X2:0 Y1:050 Y2:100", text: "Text", wantSRT: coordinates(0, 0, 50, 100)},
		{
			name: "a coordinate above the int64 range saturates", rest: " X1:99999999999999999999 X2:1 Y1:2 Y2:3", text: "Text",
			wantSRT: coordinates(9223372036854775807, 1, 2, 3),
		},
		{name: "2 settings", rest: " align:start position:0%", text: "Text", wantVTT: settings([2]string{"align", "start"}, [2]string{"position", "0%"})},
		{name: "2 spaces", rest: "  align:start", text: "Text", wantVTT: settings([2]string{"align", "start"})},
		{
			name: "every setting", rest: " vertical:rl line:0 position:10% size:80% align:end", text: "Text",
			wantVTT: settings([2]string{"vertical", "rl"}, [2]string{"line", "0"}, [2]string{"position", "10%"}, [2]string{"size", "80%"}, [2]string{"align", "end"}),
		},
		{
			name: "a repeated setting keeps its first place", rest: " align:start line:0 align:end", text: "Text",
			wantVTT: settings([2]string{"align", "end"}, [2]string{"line", "0"}),
		},
		{
			name: "settings with an alignment tag", rest: " align:start", text: `{\an8}Text`,
			wantVTT: model.NewData(model.Field{Key: "align", Value: "start"}, model.Field{Key: "alignment", Value: int64(8)}),
		},
		{
			name: "settings with a bottom center tag", rest: " align:start", text: `{\an2}Text`,
			wantVTT: model.NewData(model.Field{Key: "align", Value: "start"}, model.Field{Key: "alignment", Value: int64(2)}),
		},
		{name: "an alignment tag without settings", rest: "", text: `{\an8}Text`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, "1\n00:00:01,000 --> 00:00:03,500"+tt.rest+"\n"+tt.text+"\n", false)
			c := firstCue(t, s)
			assert.Equal(t, 3500*ms, c.End)
			assert.Equal(t, []string{"Text"}, c.Lines)
			gotSRT, _ := c.FormatData("srt")
			gotVTT, _ := c.FormatData("vtt")
			assert.Equal(t, tt.wantSRT, gotSRT)
			assert.Equal(t, tt.wantVTT, gotVTT)
			assert.Empty(t, warnings)
		})
	}
}

// Ports testLenientModeIgnoresUnknownTextAfterTheEndTimeAndWarns of tests/Parsers/SubRipParserTest.php of subtitle-toolbox-php.
func TestReadLenientUnknownTextAfterEndTime(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "1\n00:00:01,000 --> 00:00:03,500 align:start foo bar\nText\n\n2\n00:00:04,000 --> 00:00:05,000\nMore\n", true)
	require.Equal(t, 2, s.Len())
	vtt, _ := firstCue(t, s).FormatData("vtt")
	assert.Equal(t, model.NewData(model.Field{Key: "align", Value: "start"}), vtt)
	assert.Equal(t, []warningRow{{`Block #0 has the unknown text "foo bar" after the end time. The parser ignored it.`, 2, 0, model.Repaired}},
		warningRows(warnings))
	assert.Equal(t, []string{"1", "00:00:01,000 --> 00:00:03,500 align:start foo bar", "Text"}, warnings[0].Lines)
}

// Ports testStrictModeReadsOnlyTheArrowWithTwoDashes and testLenientModeReadsAnyArrowAndWarnsForOtherLengths
// of tests/Parsers/SubRipParserTest.php of subtitle-toolbox-php.
func TestReadArrows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		timing      string
		strictReads bool
		arrow       string
	}{
		{"no spaces", "00:00:01,000-->00:00:02,000", true, ""},
		{"two spaces", "00:00:01,000  -->  00:00:02,000", true, ""},
		{"tabs", "00:00:01,000\t-->\t00:00:02,000", true, ""},
		{"short arrow", "00:00:01,000 -> 00:00:02,000", false, "->"},
		{"long arrow", "00:00:01,000 ---> 00:00:02,000", false, "--->"},
		{"long, no space", "00:00:01,000--->00:00:02,000", false, "--->"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := "1\n" + tt.timing + "\nText\n\n2\n00:00:03,000 --> 00:00:04,000\nMore\n"
			first := cueRow{time.Second, 2 * time.Second, []string{"Text"}}
			if tt.strictReads {
				s, _ := mustRead(t, content, false)
				assert.Equal(t, first, cueRows(s)[0])
			} else {
				perr := readError(t, content)
				assert.Equal(t, `Block #0 has no timing line on its second line. The line is "`+strings.ReplaceAll(tt.timing, "\t", " ")+`".`, perr.Msg)
			}

			s, warnings := mustRead(t, content, true)
			require.Equal(t, 2, s.Len())
			assert.Equal(t, first, cueRows(s)[0])
			if tt.arrow == "" {
				assert.Empty(t, warnings)
				return
			}
			msg := `Block #0 has the arrow "` + tt.arrow + `" in its timing line. The parser read it as "-->".`
			assert.Equal(t, []warningRow{{msg, 2, 0, model.Repaired}}, warningRows(warnings))
		})
	}
}

// The arrow is the first "-->" that does not directly follow a "-", as the PHP pattern (.*?)(?<!-)\s*(-->) reads it.
func TestReadArrowAfterADash(t *testing.T) {
	t.Parallel()

	content := "1\n00:00:01,000- --> 00:00:02,000\nText\n"
	perr := readError(t, content)
	assert.Equal(t, `The time "00:00:01,000-" is not valid.`, perr.Msg)
	assert.Equal(t, 2, perr.Line)

	_, warnings := mustRead(t, content, true)
	assert.Equal(t, []warningRow{{`The time "00:00:01,000-" is not valid.`, 2, 0, model.Skipped}}, warningRows(warnings))
}

// Rows of the join and split rules of https://github.com/yama6a/subtitle-toolbox-go/issues/20.
func TestReadJoinAndSplit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string
		want         []cueRow
		wantLenient  []warningRow
		wantStrictOK bool
	}{
		{
			name:         "text after an empty line stays in the cue",
			content:      "1\n00:00:01,000 --> 00:00:02,000\nFirst\n\nsecond line\n\n2\n00:00:03,000 --> 00:00:04,000\nNext\n",
			want:         []cueRow{{time.Second, 2 * time.Second, []string{"First", "second line"}}, {3 * time.Second, 4 * time.Second, []string{"Next"}}},
			wantLenient:  []warningRow{{"Block #0 has an empty line before line 5 inside the cue. The parser kept the text after it in the cue.", 5, 0, model.Repaired}},
			wantStrictOK: true,
		},
		{
			name:         "a number line after a timing line goes with the next timing line",
			content:      "1\n00:00:01,000 --> 00:00:02,000\n42\n00:00:03,000 --> 00:00:04,000\nText\n",
			want:         []cueRow{{time.Second, 2 * time.Second, nil}, {3 * time.Second, 4 * time.Second, []string{"Text"}}},
			wantLenient:  []warningRow{{"Block #0 has no empty line before line 3. The parser split the block there.", 3, 0, model.Repaired}},
			wantStrictOK: true,
		},
		{
			name:         "a number line of cue text",
			content:      "1\n00:00:01,000 --> 00:00:02,000\n42\nText\n",
			want:         []cueRow{{time.Second, 2 * time.Second, []string{"42", "Text"}}},
			wantStrictOK: true,
		},
		{
			name:    "2 cues without an empty line",
			content: "1\n00:00:01,000 --> 00:00:02,000\nOne\n2\n00:00:03,000 --> 00:00:04,000\nTwo\n",
			want:    []cueRow{{time.Second, 2 * time.Second, []string{"One"}}, {3 * time.Second, 4 * time.Second, []string{"Two"}}},
			wantLenient: []warningRow{
				{"Block #0 has no empty line before line 4. The parser split the block there.", 4, 0, model.Repaired},
			},
			wantStrictOK: true,
		},
		{
			name:    "a broken time after an empty line stays apart",
			content: "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n00:00:03 to 00:00:04\nTwo\n",
			want:    []cueRow{{time.Second, 2 * time.Second, []string{"One"}}},
			wantLenient: []warningRow{
				{`Block #1 has no cue number on its first line. The line is "00:00:03 to 00:00:04".`, 5, 1, model.Skipped},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := mustRead(t, tt.content, true)
			assert.Equal(t, tt.want, cueRows(s))
			assert.Equal(t, tt.wantLenient, warningRows(warnings))

			if !tt.wantStrictOK {
				_ = readError(t, tt.content)
				return
			}
			s, warnings = mustRead(t, tt.content, false)
			assert.Equal(t, tt.want, cueRows(s))
			assert.Empty(t, warnings)
		})
	}
}

// Rows of the warning order of https://github.com/yama6a/subtitle-toolbox-go/issues/20.
func TestReadWarningOrder(t *testing.T) {
	t.Parallel()

	content := "Banner\n\n" +
		"0:0:2\uff0c500 -> 0:0:1,5 foo\nText\n\nmore\n"
	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{{1500 * ms, 2500 * ms, []string{"Text", "more"}}}, cueRows(s))
	timing := "0:0:2\uff0c500 -> 0:0:1,5 foo"
	assert.Equal(t, []warningRow{
		{`Block #0 has no cue number on its first line. The line is "Banner".`, 1, 0, model.Skipped},
		{"Block #1 has an empty line before line 6 inside the cue. The parser kept the text after it in the cue.", 6, 1, model.Repaired},
		{"The cue ends at 1.5 s, before it starts at 2.5 s. The parser swapped the times.", 3, 1, model.Repaired},
		{`Block #1 has the unknown text "foo" after the end time. The parser ignored it.`, 3, 1, model.Repaired},
		{`Block #1 has the time "0:0:2,500", which is not in the form hh:mm:ss,mmm. The parser read it as 2.5 s.`, 3, 1, model.Repaired},
		{`Block #1 has the time "0:0:1,5", which is not in the form hh:mm:ss,mmm. The parser read it as 1.5 s.`, 3, 1, model.Repaired},
		{"Block #1 has full-width delimiters in its timing line. The parser read them as ASCII.", 3, 1, model.Repaired},
		{`Block #1 has the arrow "->" in its timing line. The parser read it as "-->".`, 3, 1, model.Repaired},
		{"Block #1 has no cue number on line 3. The parser read the cue without it.", 3, 1, model.Repaired},
	}, warningRows(warnings))
	for _, w := range warnings[2:] {
		assert.Equal(t, []string{timing, "Text", "more"}, w.Lines)
	}
}

func TestReadInvalidUTF8(t *testing.T) {
	t.Parallel()

	content := "1\n00:00:01,000 --> 00:00:02,000\ncaf\xe9 & <i>cr\xe8me</i>\n"
	s, warnings := mustRead(t, content, false)
	assert.Equal(t, []string{"caf\xe9 &amp; <i>cr\xe8me</i>"}, firstCue(t, s).Lines)
	assert.Empty(t, warnings)

	_, warnings = mustRead(t, content, true)
	assert.Equal(t, []warningRow{{
		"The content is not valid UTF-8. The first bad byte is at offset 35. Set the source encoding of the file.", model.NoLine, model.NoBlock, model.Repaired,
	}}, warningRows(warnings))
}

func TestReadNUL(t *testing.T) {
	t.Parallel()

	perr := readError(t, "1\n00:00:01,000 --> 00:00:02,000\nA\x00B\n")
	assert.Equal(t, "Line 3 has a NUL character. Check the encoding of the file.", perr.Msg)

	s, _ := mustRead(t, "1\n00:00:01,000 --> 00:00:02,000\nA\x00B\n", true)
	assert.Equal(t, []string{"AB"}, firstCue(t, s).Lines)
}
