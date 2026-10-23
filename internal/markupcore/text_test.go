package markupcore_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

func TestRubyAsText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{"one annotation", "<ruby>K<rt>kan</rt></ruby>", "K (kan)"},
		{"two annotations", "<ruby>K<rt>kan</rt>J<rt>ji</rt></ruby>", "K (kan)J (ji)"},
		{"rp elements are dropped", "<ruby>K<rp>(</rp><rt>kan</rt><rp>)</rp></ruby>", "K (kan)"},
		{"rt end tag left out", "<ruby>K<rt>kan</ruby> end", "K (kan) end"},
		{"class and space in base", "<ruby.big>ruby <rt>rubyText</rt></ruby>text", "ruby (rubyText)text"},
		{"empty annotation", "<ruby>K<rt></rt></ruby>", "K"},
		{"blank annotation keeps the base as written", "<ruby>K <rt> </rt> L</ruby>", "K  L"},
		{"no ruby", "<i>a</i> &lt;ruby&gt;", "<i>a</i> &lt;ruby&gt;"},
		{"ruby end tag left out before a final LF", "<ruby>K<rt>kan</ruby> end\n", "K (kan) end\n"},
		{"ruby left open before a final LF", "<ruby>K<rt>kan\n", "K (kan)\n"},
		{"any case and spaces in end tags", "<RUBY>K<RT> kan </RT ></RUBY >x", "K (kan)x"},
		{"longer tag name", "a<rubyx>b", "a<rubyx>b"},
		{"several ruby spans", "<ruby>K<rt>a</rt>L<rt>b</rt></ruby> <ruby>M<rt>c</rt></ruby>", "K (a)L (b) M (c)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.RubyAsText(tt.text))
		})
	}
}

// TestRubyFixture reads own_ruby.vtt and compares the ruby text with the expected SubRip and plain text output.
func TestRubyFixture(t *testing.T) {
	t.Parallel()

	read := func(name string) string {
		t.Helper()
		return string(testfiles.Read(t, "ruby/"+name))
	}
	vttCues := cueTexts(read("own_ruby.vtt"))
	srtCues := cueTexts(read("own_ruby_expected.srt"))
	require.Len(t, vttCues, 3)

	got := make([]string, 0, len(vttCues))
	plain := make([]string, 0, len(vttCues))
	for _, lines := range vttCues {
		cue := make([]string, 0, len(lines))
		for _, line := range lines {
			cue = append(cue, markupcore.RubyAsText(line))
		}
		got = append(got, strings.Join(cue, "\n"))
		plain = append(plain, markupcore.PlainLines(lines)...)
	}
	want := make([]string, 0, len(srtCues))
	for _, lines := range srtCues {
		want = append(want, strings.Join(lines, "\n"))
	}
	assert.Equal(t, want, got)
	assert.Equal(t, read("own_ruby_expected.txt"), strings.Join(plain, " ")+"\n")
}

// cueTexts returns the text lines of each block of a SubRip or WebVTT file, the lines after the timing line.
func cueTexts(content string) [][]string {
	var cues [][]string
	for block := range strings.SplitSeq(strings.TrimSpace(content), "\n\n") {
		lines := strings.Split(block, "\n")
		for i, line := range lines {
			if strings.Contains(line, "-->") {
				cues = append(cues, lines[i+1:])
				break
			}
		}
	}
	return cues
}

func TestPlainText(t *testing.T) {
	t.Parallel()

	assert.Equal(t, " Tom & Jerry <3 ", markupcore.PlainText(" <i>Tom &amp; Jerry</i> &lt;3<00:00:01.000> "))
	assert.Equal(t, "<i>", markupcore.PlainText("&lt;i&gt;"))
}

func TestPlainLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{
			"strips, decodes, trims and drops empty lines",
			[]string{" <i>Tom &amp; Jerry</i> ", "<b></b>", "&lt;not a tag&gt;", "  ", "Caf&eacute;<00:00:01.000>"},
			[]string{"Tom & Jerry", "<not a tag>", "Caf\u00e9"},
		},
		{"no lines", nil, nil},
		{"quote in a voice tag", []string{"<v O'Neil>We're out.", "<i></i>"}, []string{"We're out."}},
		{"ruby", []string{"<ruby>K<rt>kan</rt></ruby> end"}, []string{"K (kan) end"}},
		{"line breaks", []string{"<i>a\n b</i>", "c\r\nd", "e&#10;f&NewLine;"}, []string{"a", "b", "c", "d", "e", "f"}},
		{"entity of an entity", []string{"&amp;amp;"}, []string{"&amp;"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.PlainLines(tt.lines))
		})
	}
}

func TestVisibleLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want int
	}{
		{"tags, outer spaces and entities", " <i>Gr\u00fc\u00dfe &amp; </i><00:00:01.000>Tee ", 11},
		{"no text", "<b></b> ", 0},
		{"quote in a voice tag", "<v O'Neil>We're out.", 10},
		{"docs example", "<i>Caf\u00e9</i> &amp; tea ", 10},
		{"no-break space is not trimmed", "\u00a0a\u00a0", 3},
		{"invalid UTF-8 counts bytes", "Caf\xe9", 4},
		{"combining mark", "<i>Cafe\u0301</i>", 4},
		{"emoji sequence", "\U0001f468\u200d\U0001f469\u200d\U0001f467 \U0001f44d\U0001f3fd", 3},
		{"bidi marks count 0", "\u202bab\u202c", 2},
		{"entity of a combining mark", "e&#769;", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.VisibleLength(tt.text))
		})
	}
}

func TestToSingleLine(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Line one Line two", markupcore.ToSingleLine("  Line one  \r\n  Line two\n"))
	assert.Equal(t, "a b c", markupcore.ToSingleLine("a \n \n b\v\n\vc"))
}

func TestWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{"ASCII white space", " Hello,\tworld!\nBye ", []string{"Hello,", "world!", "Bye"}},
		{"invalid UTF-8", "Caf\xe9 au  lait", []string{"Caf\xe9", "au", "lait"}},
		{"invalid UTF-8 keeps the bytes of a no-break space", "a\xc2\xa0b\xe9 c", []string{"a\xc2\xa0b\xe9", "c"}},
		{"only white space", "  ", []string{}},
		{"Unicode white space", "a\u00a0b\u2003c\vd\u3000e", []string{"a", "b", "c", "d", "e"}},
		{"Mongolian vowel separator is white space as in PHP", "a\u180eb", []string{"a", "b"}},
		{"zero-width space is no white space", "a\u200bb", []string{"a\u200bb"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.Words(tt.text))
		})
	}
}

func TestCharacters(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"C", "a", "f", "\u00e9"}, markupcore.Characters("Caf\u00e9"))
	assert.Equal(t, []string{"C", "a", "f", "\xe9"}, markupcore.Characters("Caf\xe9"))
	assert.Empty(t, markupcore.Characters(""))
}

func TestSpeaker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		text   string
		want   string
		wantOK bool
	}{
		{"name", "<v Fred>Hi", "Fred", true},
		{"classes, entities and spaces", "<i><v.loud  Ann &amp; Bo >Hi", "Ann & Bo", true},
		{"vertical tab before the name", "<v\vAnn>Hi", "Ann", true},
		{"quote in the name", "<v O'Neil>Hi", "O'Neil", true},
		{"first voice tag", "<v A>a <v B>b", "A", true},
		{"no name", "<v>Hi", "", false},
		{"no voice tag", "<b>Hi</b>", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := markupcore.Speaker(tt.text)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddSpeaker(t *testing.T) {
	t.Parallel()

	lines := []string{" ", "  Hi", "there"}
	assert.Equal(t, []string{" ", "<v Ann &amp; Bo>Hi", "there"}, markupcore.AddSpeaker(lines, "Ann & Bo"))
	assert.Equal(t, []string{" ", "  Hi", "there"}, lines, "the input stays unchanged")
	assert.Equal(t, lines, markupcore.AddSpeaker(lines, ""))
	assert.Equal(t, []string{" "}, markupcore.AddSpeaker([]string{" "}, "Ann"))
}

func TestVoiceTag(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "<v Ann &amp; Bo>", markupcore.VoiceTag("Ann & Bo", ""))
	assert.Equal(t, `<v.loud O'Neil "Jr" &lt;3>`, markupcore.VoiceTag(`O'Neil "Jr" <3`, ".loud"))
}

func TestMapTextRuns(t *testing.T) {
	t.Parallel()

	type call struct {
		text        string
		first, last bool
	}
	var calls []call
	lines := []string{"<i>a &amp; b</i> c", "<b></b>d"}
	got := markupcore.MapTextRuns(lines, func(text string, first, last bool) string {
		calls = append(calls, call{text, first, last})
		return strings.ToUpper(text) + "<"
	})

	assert.Equal(t, []string{"<i>A &amp; B&lt;</i> C&lt;", "<b></b>D&lt;"}, got)
	assert.Equal(t, []call{{"a & b", true, false}, {" c", false, true}, {"d", true, true}}, calls)
	assert.Equal(t, []string{"<i>a &amp; b</i> c", "<b></b>d"}, lines, "the input stays unchanged")
}

func TestMapTextRunsKeepsUnchangedRuns(t *testing.T) {
	t.Parallel()

	tests := []string{"<i>Tom & Jerry</i>", "a > b &nbsp; &AMP; &amp;lt;", "<>"}
	for _, line := range tests {
		t.Run(line, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, []string{line}, markupcore.MapTextRuns([]string{line}, func(text string, _, _ bool) string { return text }))
		})
	}
}

func TestNormalizeLineEnds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"a\nb", "a\nb"},
		{"a\r\nb\r\n", "a\nb\n"},
		{"a\r\r\nb", "a\nb"},
		{"a\rb", "a\nb"},
		{"a\r\rb", "a\n\nb"},
		{"a\r", "a\n"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.NormalizeLineEnds(tt.in))
		})
	}
}
