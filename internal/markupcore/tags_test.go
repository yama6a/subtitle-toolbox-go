package markupcore_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

func TestSplitTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want []string
	}{
		{"tags and text", "<i>Hello <00:00:01.000>world</i> &lt;3", []string{"", "<i>", "Hello ", "<00:00:01.000>", "world", "</i>", " &lt;3"}},
		{"less-than sign before a space", "a < b", []string{"a < b"}},
		{"less-than sign before a space starts no tag", "< b>", []string{"< b>"}},
		{"spaces on both sides", "a < b > c", []string{"a < b > c"}},
		{"less-than sign before a vertical tab", "<\vb>", []string{"<\vb>"}},
		{"less-than sign before a form feed", "<\fb>", []string{"<\fb>"}},
		{"empty tag", "<>", []string{"", "<>", ""}},
		{"quote inside a tag", "<v O'Neil>Hi", []string{"", "<v O'Neil>", "Hi"}},
		{"double quote inside a tag", `<v Mo "Baker>Two`, []string{"", `<v Mo "Baker>`, "Two"}},
		{"tag starts at the last less-than sign", "<<b>", []string{"<", "<b>", ""}},
		{"no tag", "plain", []string{"plain"}},
		{"empty", "", []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.SplitTags(tt.line))
		})
	}
}

func TestStripAllTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{"every tag", `<v Fred>Fred: <b>Hello</b> <font color="#ff0000">world</font> <00:01:02.500>now</v>`, "Fred: Hello world now"},
		{"escaped text stays", "<i>&lt;b&gt;1 &amp; 2&lt;/b&gt;</i>", "&lt;b&gt;1 &amp; 2&lt;/b&gt;"},
		{"single quote in a voice tag", "<v O'Neil>Hi.</v>", "Hi."},
		{"single quote in a voice tag and the text", "<v O'Neil>We're out.", "We're out."},
		{"lone double quote in a voice tag", `<v Mo "Baker>Two rolls.`, "Two rolls."},
		{"double quotes in a voice tag and the text", `<v Sam "Ace" Reed>Take the "seeded" loaf.`, `Take the "seeded" loaf.`},
		{"single quotes in an attribute", "<font color='#ff0000'>Red</font> jam", "Red jam"},
		{"quotes in the text", `It's <i>'fine'</i>, "really"`, `It's 'fine', "really"`},
		{"less-than sign before a digit", "I <3 bread", "I <3 bread"},
		{"less-than sign before a space", "a < b", "a < b"},
		{"spaces on both sides", "a < b > c", "a < b > c"},
		{"empty tag", "a<>b", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.StripAllTags(tt.text))
		})
	}
}

func TestKeepTags(t *testing.T) {
	t.Parallel()

	core := `<v Fred><b>b</b><i>i</i><u>u</u><s>s</s><font color="#ff0000">c</font></v>`
	tests := []struct {
		name  string
		text  string
		names []string
		want  string
	}{
		{"only the given tags", `<b>bold</b> <font color="#ff0000">red</font> <s>struck</s>`, []string{"b", "font"}, `<b>bold</b> <font color="#ff0000">red</font> struck`},
		{"core tags", core + "<c.yellow>x</c>", []string{"b", "i", "u", "s", "font", "v"}, core + "x"},
		{"no names", "<b>bold</b>", nil, "bold"},
		{"quote in a voice tag", "<v O'Neil><i>Hi.</i>", []string{"i"}, "<i>Hi.</i>"},
		{"quote in a kept voice tag", "<v O'Neil>We're out.", []string{"v"}, "<v O'Neil>We're out."},
		{"lone double quote in a voice tag", `<v Mo "Baker>Two rolls.`, []string{"b", "i", "u", "s", "font", "v"}, `<v Mo "Baker>Two rolls.`},
		{"single quotes in an attribute", "<font color='#ff0000'>Red</font> <s>jam</s>", []string{"font"}, "<font color='#ff0000'>Red</font> jam"},
		{"class tag is its own name", "It's <c.loud>x</c>", []string{"i"}, "It's x"},
		{"names without case", "<B>bold</B> <c.Loud>x</c.Loud> <i>y</i>", []string{"b", "C.loud"}, "<B>bold</B> <c.Loud>x</c.Loud> y"},
		{"spaces on both sides", "a < b > c", nil, "a < b > c"},
		{"spaces on both sides with the name kept", "a < b > c", []string{"b"}, "a < b > c"},
		{"word timestamp has no name", "<00:00:01.000>a", []string{"b"}, "a"},
		{"self-closing tag", "a<br/>b", []string{"br"}, "a<br/>b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.KeepTags(tt.text, tt.names...))
		})
	}
}

func TestHasVisibleText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"tags and white space", []string{"<i> </i>", "", "<00:00:01.000>"}, false},
		{"an entity counts as text", []string{"<i></i>", "<b>&nbsp;</b>"}, true},
		{"no lines", nil, false},
		{"spaces on both sides", []string{"a < b > c"}, true},
		{"less-than sign before a space", []string{"< b>"}, true},
		{"trim set", []string{" \t\n\r\x00\v<b></b>"}, false},
		{"form feed is text", []string{"\f"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.HasVisibleText(tt.lines))
		})
	}
}

func TestTagName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		body string
		want string
	}{
		{` FONT color="yellow"`, "font"},
		{"c.yellow", "c.yellow"},
		{"", ""},
		{"I", "i"},
		{"v\tAnna", "v"},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.TagName(tt.body))
		})
	}
}

func TestUnknownTagsAsText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"unknown tag alone", []string{"Hi <today>"}, []string{"Hi &lt;today&gt;"}},
		{"end tag on another line", []string{"<today>Hi", "there</today>"}, []string{"<today>Hi", "there</today>"}},
		{"end tag in another case with a space", []string{"<Today>Hi", "</TODAY >"}, []string{"<Today>Hi", "</TODAY >"}},
		{"known tags stay", []string{"<b><I><font><v><c><lang><ruby><rt><br><u><s>"}, []string{"<b><I><font><v><c><lang><ruby><rt><br><u><s>"}},
		{"tag with attributes stays", []string{`<today x="1">`}, []string{`<today x="1">`}},
		{"tag that starts with a digit stays", []string{"<3d>"}, []string{"<3d>"}},
		{"each line", []string{"<x1>a", "<y>b"}, []string{"&lt;x1&gt;a", "&lt;y&gt;b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.UnknownTagsAsText(tt.lines))
		})
	}
}

func TestRemoveEmptyTagPairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		text       string
		ignoreCase bool
		withSpaces bool
		want       string
	}{
		{"pair", "a<i></i>b", false, false, "ab"},
		{"nested pairs", "<b><i></i></b>", false, false, ""},
		{"attributes and classes", `<font color="#ff0000"></font><b.loud></b>`, false, false, ""},
		{"other names stay", "<bold></bold><v A></v>", false, false, "<bold></bold><v A></v>"},
		{"case kept apart", "<I></I><i></I>", false, false, "<I></I><i></I>"},
		{"case ignored", "<I></I><i></I>", true, false, ""},
		{"white space stays without the flag", "<i> </i>", false, false, "<i> </i>"},
		{"white space stays and the pair goes", "<b><i> </i>\v</b>", false, true, " \v"},
		{"text stays", "<i>a</i>", false, true, "<i>a</i>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.RemoveEmptyTagPairs(tt.text, tt.ignoreCase, tt.withSpaces))
		})
	}
}

func TestUnbalancedTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lines     []string
		names     []string
		wantStray []markupcore.TagSpan
		wantOpen  []string
	}{
		{
			"stray and open tags",
			[]string{"</i><b>a<i>b", "</B></i><u>"},
			[]string{"b", "i", "u"},
			[]markupcore.TagSpan{{Line: 0, Offset: 0, Length: 4}},
			[]string{"u"},
		},
		{
			"a closing tag closes the last open tag of its name",
			[]string{"<i>a<b>b<i>c</i>"},
			[]string{"b", "i"},
			nil,
			[]string{"i", "b"},
		},
		{
			"classes and attributes",
			[]string{`<c.yellow>a</c> <font color="red">b</FONT >`, "</c>"},
			[]string{"c", "font"},
			[]markupcore.TagSpan{{Line: 1, Offset: 0, Length: 4}},
			[]string{},
		},
		{
			"longer names are other tags",
			[]string{"<br><bold></b>"},
			[]string{"b"},
			[]markupcore.TagSpan{{Line: 0, Offset: 10, Length: 4}},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stray, open := markupcore.UnbalancedTags(tt.lines, tt.names)
			assert.Equal(t, tt.wantStray, stray)
			assert.Equal(t, tt.wantOpen, open)
		})
	}
}

func TestOpenCoreTags(t *testing.T) {
	t.Parallel()

	open := markupcore.OpenCoreTags(`<I>a <font color="#ff0000">b <c.x>c</c> <b>d</b>`, nil)
	assert.Equal(t, []markupcore.OpenTag{{Name: "i", Tag: "<I>"}, {Name: "font", Tag: `<font color="#ff0000">`}}, open)

	after := markupcore.OpenCoreTags("e</font>", open)
	assert.Equal(t, []markupcore.OpenTag{{Name: "i", Tag: "<I>"}}, after)
	assert.Len(t, open, 2, "the input stays unchanged")

	assert.Empty(t, markupcore.OpenCoreTags("</u>plain", nil))
	assert.Equal(t, []markupcore.OpenTag{{Name: "v", Tag: "<v.loud Anna>"}, {Name: "b", Tag: "<b.x>"}},
		markupcore.OpenCoreTags("<v.loud Anna><b.x><bold><00:00:01.000>", nil))
}

func TestCloseCoreTags(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "</font></i>", markupcore.CloseCoreTags(markupcore.OpenCoreTags(`<i>a <font color="#ff0000">b`, nil)))
	assert.Empty(t, markupcore.CloseCoreTags(nil))
}

func TestCarryOpenTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"open tag", []string{"<i>a", "b</i>", "c"}, []string{"<i>a", "<i>b</i>", "c"}},
		{"color class", []string{"<c.yellow>a", "b</c>"}, []string{`<font color="#ffff00">a`, `<font color="#ffff00">b</font>`}},
		{"tags as written", []string{"<B><v Ann>a", "b"}, []string{"<B><v Ann>a", "<B><v Ann>b"}},
		{"closed on the same line", []string{"<i>a</i>", "b"}, []string{"<i>a</i>", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.CarryOpenTags(tt.lines))
		})
	}
}
