package ttml_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// styledRoot is a root start tag that declares tts and ttm, without its closing >.
const styledRoot = `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling" xmlns:ttm="http://www.w3.org/ns/ttml#metadata"`

const (
	oneParagraph = `<p begin="1s" end="2s">x</p>`
	// bottomCenterRegion is the region that the writer adds for BottomCenter.
	bottomCenterRegion = `<region xml:id="bottomCenter" tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="after" tts:textAlign="center"/>`
	topCenterRegion    = `<region xml:id="topCenter" tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="before" tts:textAlign="center"/>`
	imscRoot           = ` xmlns:i="http://www.w3.org/ns/ttml/profile/imsc1#styling"`
	fixedRegion        = `<layout><region xml:id="r" tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="after"/></layout>`
)

func styledDocument(head, body, rootAttributes string) string {
	return styledRoot + rootAttributes + ">" + head + "<body>" + body + "</body></tt>"
}

// writeRule is a write case: TTML that the reader reads, a change of the cues, and parts of the output.
type writeRule struct {
	name   string
	in     string
	change func(t *testing.T, s *model.Subtitle)
	want   []string
}

func alignTo(a model.Alignment) func(*testing.T, *model.Subtitle) {
	return func(t *testing.T, s *model.Subtitle) {
		t.Helper()
		setCue(t, s, 0, func(c *model.Cue) { c.Alignment = a })
	}
}

func forceTo(i int, forced bool) func(*testing.T, *model.Subtitle) {
	return func(t *testing.T, s *model.Subtitle) {
		t.Helper()
		setCue(t, s, i, func(c *model.Cue) { c.Forced = forced })
	}
}

func linesTo(lines ...string) func(*testing.T, *model.Subtitle) {
	return func(t *testing.T, s *model.Subtitle) {
		t.Helper()
		setCue(t, s, 0, func(c *model.Cue) { c.Lines = lines })
	}
}

func titleTo(title string) func(*testing.T, *model.Subtitle) {
	return func(_ *testing.T, s *model.Subtitle) { s.SetMetadata(model.MetadataTitle, title) }
}

func runWriteRules(t *testing.T, tests []writeRule) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.in, false)
			if tt.change != nil {
				tt.change(t, s)
			}
			out := writeDefault(t, s)
			for _, want := range tt.want {
				assert.Contains(t, out, want)
			}
		})
	}
}

func TestInsertRule(t *testing.T) {
	t.Parallel()

	body := "<div>" + oneParagraph + "</div>"
	runWriteRules(t, []writeRule{
		{"title in an empty head", styledDocument("<head/>", body, ""), titleTo("T"), []string{"\n  <head>\n    <ttm:title>T</ttm:title>\n  </head>\n"}},
		{
			"title in an indented head", styledDocument("<head>\n    <styling/>\n  </head>", body, ""), titleTo("T"),
			[]string{"\n  <head>\n    <ttm:title>T</ttm:title>\n    <styling/>\n  </head>\n"},
		},
		{
			"title before an element", styledDocument("<head><styling/></head>", body, ""), titleTo("T"),
			[]string{"\n  <head>\n    <ttm:title>T</ttm:title><styling/></head>\n"},
		},
		{
			"new layout at the end", styledDocument("<head>\n    <styling/>\n  </head>", body, ""), alignTo(model.TopCenter),
			[]string{"\n  <head>\n    <styling/>\n    <layout>\n      " + topCenterRegion + "\n    </layout>\n  </head>\n"},
		},
		{
			"new region in an empty layout", styledDocument("<head><layout/></head>", body, ""), alignTo(model.TopCenter),
			[]string{"\n  <head><layout>\n      " + topCenterRegion + "\n    </layout></head>\n"},
		},
		{
			"agent before styling", styledDocument("<head>\n    <metadata/>\n    <styling/>\n  </head>", body, ""), linesTo("<v Ann>x"),
			[]string{"\n  <head>\n    <metadata/>\n    <ttm:agent xml:id=\"agent1\" type=\"person\"><ttm:name type=\"full\">Ann</ttm:name></ttm:agent>\n    <styling/>\n  </head>\n"},
		},
		{
			"agent at the end", styledDocument("<head><metadata/></head>", body, ""), linesTo("<v Ann>x"),
			[]string{"\n  <head><metadata/>\n    <ttm:agent xml:id=\"agent1\" type=\"person\"><ttm:name type=\"full\">Ann</ttm:name></ttm:agent>\n  </head>\n"},
		},
	})
}

func TestPrefixBinding(t *testing.T) {
	t.Parallel()

	paragraph := "<body><div>" + oneParagraph + "</div></body></tt>"
	runWriteRules(t, []writeRule{
		{"TTML", `<tt xmlns="http://www.w3.org/ns/ttml">` + paragraph, nil, []string{"\n" + ttmlRoot + ">\n"}},
		{"no namespace", `<tt>` + paragraph, nil, []string{"\n" + ttmlRoot + ">\n"}},
		{
			"DFXP 2006/10", `<tt xmlns="http://www.w3.org/2006/10/ttaf1">` + paragraph, nil,
			[]string{`<tt xmlns="http://www.w3.org/2006/10/ttaf1" xmlns:ttm="http://www.w3.org/2006/10/ttaf1#metadata" xmlns:tts="http://www.w3.org/2006/10/ttaf1#style" xml:lang="">`},
		},
		{
			"DFXP 2006/04", `<tt xmlns="http://www.w3.org/2006/04/ttaf1">` + paragraph, nil,
			[]string{`<tt xmlns="http://www.w3.org/2006/04/ttaf1" xmlns:ttm="http://www.w3.org/2006/04/ttaf1#metadata" xmlns:tts="http://www.w3.org/2006/04/ttaf1#styling" xml:lang="">`},
		},
		{
			"tts taken by another URI", `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="urn:other" xmlns:tts2="urn:two">` + paragraph, alignTo(model.TopCenter),
			[]string{
				`<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" xmlns:tts="urn:other" xmlns:tts2="urn:two" xmlns:tts3="http://www.w3.org/ns/ttml#styling" xml:lang="">`,
				`<region xml:id="topCenter" tts3:origin="10% 10%" tts3:extent="80% 80%" tts3:displayAlign="before" tts3:textAlign="center"/>`,
			},
		},
	})
}

func TestUniqueIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    func(t *testing.T) *model.Subtitle
		want []string
	}{
		{"2 cues c1", func(t *testing.T) *model.Subtitle {
			t.Helper()
			s, _ := mustRead(t, styledDocument("<head/>", `<div><p xml:id="c1" begin="1s" end="2s">a</p><p xml:id="c1" begin="3s" end="4s">b</p></div>`, ""), false)
			return s
		}, []string{"c1", "c1_2"}},
		{"a cue topCenter next to a new region", func(t *testing.T) *model.Subtitle {
			t.Helper()
			return readVTT(t, "WEBVTT\n\ntopCenter\n00:00:01.000 --> 00:00:02.000 line:0\nOne\n")
		}, []string{"topCenter", "topCenter_2"}},
		{"a stored root xml:id", func(t *testing.T) *model.Subtitle {
			t.Helper()
			s, _ := mustRead(t, styledDocument(`<head><styling><style xml:id="a"/></styling></head>`, `<div><p xml:id="a" begin="1s" end="2s">x</p></div>`, ` xml:id="a"`), false)
			return s
		}, []string{"a_2", "a", "a_3"}},
		{"a cue c1 plus LF", func(*testing.T) *model.Subtitle {
			c := cue(time.Second, 2*time.Second, "x")
			c.ID = "c1\n"
			return subtitleOf(c)
		}, []string{"bottomCenter"}},
		{"cue IDs that are no NCName", func(t *testing.T) *model.Subtitle {
			t.Helper()
			return readVTT(t, "WEBVTT\n\n1 a\n00:00:01.000 --> 00:00:02.000\nOne\n\nb.c-d_1\n00:00:03.000 --> 00:00:04.000\nTwo\n")
		}, []string{"bottomCenter", "b.c-d_1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ids(writeDefault(t, tt.s(t))))
		})
	}
}

// Covers the TTML rows of the alignment rule of https://github.com/yama6a/subtitle-toolbox-go/issues/12.
func TestRealign(t *testing.T) {
	t.Parallel()

	region := `<layout><region xml:id="r" tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="before" tts:textAlign="center"/></layout>`
	runWriteRules(t, []writeRule{
		{
			"unchanged keeps the stored region", styledDocument("<head>"+region+"</head>", `<div><p begin="1s" end="2s" region="r">x</p></div>`, ""), nil,
			[]string{`<p begin="00:00:01.000" end="00:00:02.000" region="r">x</p>`, "<head>" + region + "</head>"},
		},
		{
			"changed drops the stored region", styledDocument("<head>"+region+"</head>", `<div><p begin="1s" end="2s" region="r">x</p></div>`, ""), alignTo(model.BottomCenter),
			[]string{`<p begin="00:00:01.000" end="00:00:02.000" region="bottomCenter">x</p>`, bottomCenterRegion},
		},
		{
			"tts:textAlign added", styledDocument(`<head><styling><style xml:id="s" tts:textAlign="center"/></styling>`+fixedRegion+"</head>",
				`<div><p begin="1s" end="2s" style="s" region="r">x</p></div>`, ""), alignTo(model.TopRight),
			[]string{`<p begin="00:00:01.000" end="00:00:02.000" style="s" tts:textAlign="right" region="topRight">x</p>`},
		},
		{
			"tts:textAlign replaced", styledDocument("<head>"+fixedRegion+"</head>", `<div><p begin="1s" end="2s" region="r" tts:textAlign="center" tts:color="red">x</p></div>`, ""),
			alignTo(model.TopLeft),
			[]string{`<p begin="00:00:01.000" end="00:00:02.000" tts:textAlign="left" tts:color="red" region="topLeft"><span tts:color="#ff0000">x</span></p>`},
		},
		{
			"tts:textAlign of the div", styledDocument("<head>"+fixedRegion+"</head>", `<div tts:textAlign="center"><p begin="1s" end="2s" region="r">x</p></div>`, ""),
			alignTo(model.BottomLeft),
			[]string{`<p begin="00:00:01.000" end="00:00:02.000" tts:textAlign="left" region="bottomLeft">x</p>`},
		},
		{
			"tts:textAlign that matches", styledDocument("<head>"+fixedRegion+"</head>", `<div><p begin="1s" end="2s" region="r" tts:textAlign="center">x</p></div>`, ""),
			alignTo(model.TopCenter),
			[]string{`<p begin="00:00:01.000" end="00:00:02.000" tts:textAlign="center" region="topCenter">x</p>`},
		},
		{
			"BottomCenter in a subtitle with ttml data", styledDocument("<head/>", "<div>"+oneParagraph+"</div>", ""), nil,
			[]string{`<p begin="00:00:01.000" end="00:00:02.000">x</p>`, "<head/>"},
		},
	})

	srtOut := writeDefault(t, readSRT(t, "1\n00:00:01,000 --> 00:00:02,000\nx\n"))
	assert.Contains(t, srtOut, `<p begin="00:00:01.000" end="00:00:02.000" region="bottomCenter">x</p>`, "BottomCenter in a subtitle without ttml data")
}

func TestForcedFlag(t *testing.T) {
	t.Parallel()

	runWriteRules(t, []writeRule{
		{
			"equal flag", styledDocument("<head/>", `<div><p begin="1s" end="2s" i:forcedDisplay="true">x</p></div>`, imscRoot), nil,
			[]string{`<p begin="00:00:01.000" end="00:00:02.000" i:forcedDisplay="true">x</p>`},
		},
		{
			"stored p attribute changed in place", styledDocument("<head/>", `<div><p begin="1s" end="2s" i:forcedDisplay=" true " tts:color="red">x</p></div>`, imscRoot),
			forceTo(0, false),
			[]string{`<p begin="00:00:01.000" end="00:00:02.000" i:forcedDisplay="false" tts:color="red">`},
		},
		{
			"flag from a div", styledDocument("<head/>", `<div i:forcedDisplay="true"><p begin="1s" end="2s">x</p><p begin="3s" end="4s">y</p></div>`, imscRoot),
			forceTo(1, false),
			[]string{
				`<div i:forcedDisplay="true">` + "\n" + `      <p begin="00:00:01.000" end="00:00:02.000">x</p>`,
				`<p begin="00:00:03.000" end="00:00:04.000" i:forcedDisplay="false">y</p>`,
			},
		},
		{"flag from a region", styledDocument(`<head><layout><region xml:id="r" i:forcedDisplay="true"/><region xml:id="r" i:forcedDisplay="false"/></layout></head>`,
			`<div><p begin="1s" end="2s" region="r">x</p><p begin="3s" end="4s" region="r">y</p></div>`, imscRoot), forceTo(1, false), []string{
			`<p begin="00:00:01.000" end="00:00:02.000" region="r">x</p>`,
			`<p begin="00:00:03.000" end="00:00:04.000" region="r" i:forcedDisplay="false">y</p>`,
		}},
		{"new itts binding", styledDocument("<head/>", "<div>"+oneParagraph+"</div>", ` xmlns:itts="urn:other"`), forceTo(0, true), []string{
			`xmlns:itts="urn:other" xmlns:itts2="http://www.w3.org/ns/ttml/profile/imsc1#styling" xmlns:ttm=`,
			`<p begin="00:00:01.000" end="00:00:02.000" itts2:forcedDisplay="true">x</p>`,
		}},
	})
}

func TestContent(t *testing.T) {
	t.Parallel()

	in := styledDocument("<head/>", "<div>"+oneParagraph+"</div>", "")
	agent := func(id, name string) string {
		return `<ttm:agent xml:id="` + id + `" type="person"><ttm:name type="full">` + name + `</ttm:name></ttm:agent>`
	}
	p := `<p begin="00:00:01.000" end="00:00:02.000">`
	runWriteRules(t, []writeRule{
		{
			"paragraph agent", in, linesTo(" <v.loud Ann &amp; Bo>Hi <i>x</i>"),
			[]string{agent("agent1", "Ann &amp; Bo"), `<p begin="00:00:01.000" end="00:00:02.000" ttm:agent="agent1">Hi <span tts:fontStyle="italic">x</span></p>`},
		},
		{
			"paragraph agent with an empty name", in, linesTo("<v >hi"),
			[]string{`<ttm:name type="full"></ttm:name>`, `<p begin="00:00:01.000" end="00:00:02.000" ttm:agent="agent1">hi</p>`},
		},
		{"closed voice is a span", in, linesTo("<v Ann>a</v> b"), []string{p + `<span ttm:agent="agent1">a</span> b</p>`}},
		{"voice after text is a span", in, linesTo("x <v Ann>hi"), []string{p + `x <span ttm:agent="agent1">hi</span></p>`}},
		{
			"second voice is a span", in, linesTo("<v Ann>Hi</v> <v Bo>Yo <v>x</v>"),
			[]string{agent("agent1", "Ann"), agent("agent2", "Bo"), p + `<span ttm:agent="agent1">Hi</span> <span ttm:agent="agent2">Yo </span>x</p>`},
		},
		{
			"mis-nested markup", in, linesTo("<b>a<i>b</b>c</i>d</u>"),
			[]string{p + `<span tts:fontWeight="bold">a<span tts:fontStyle="italic">b</span></span><span tts:fontStyle="italic">c</span>d</p>`},
		},
		{"end tags without an open entry", in, linesTo("a</b><b >c</B >d<br>e</ x>"), []string{p + `a<span tts:fontWeight="bold">c</span>de</p>`}},
		{
			"color names in mixed case", in, linesTo(`<font color="YeLLow">a</font><FONT COLOR=#ABC>b</FONT><font face=x>c</font><font color=' Red '>d</font>`),
			[]string{p + `<span tts:color="yellow">a</span><span tts:color="#aabbcc">b</span>c<span tts:color="red">d</span></p>`},
		},
		{
			"rgba and rgb", in, linesTo(`<font color="rgba(255,0,0,0.5)">a</font> <font color="rgb(1, 2, 3)">b</font>`),
			[]string{p + `<span tts:color="#ff000080">a</span> <span tts:color="rgb(1, 2, 3)">b</span></p>`},
		},
		{"a value that is no color", in, linesTo(`<font color="bogus&amp;">a</font>`), []string{p + `<span tts:color="bogus&amp;">a</span></p>`}},
		{"entity in a color", in, linesTo(`<font color="&#35;FF0000">a</font>`), []string{p + `<span tts:color="#FF0000">a</span></p>`}},
		{
			"WebVTT class colors", in, linesTo("<c.yellow.bg_black>a</c> <c.loud>b</c> <c.red.bg_blue foo>c</c>"),
			[]string{p + `<span tts:color="#ffff00" tts:backgroundColor="#000000">a</span> b <span tts:color="#ff0000" tts:backgroundColor="#0000ff">c</span></p>`},
		},
		{"tags without TTML style", in, linesTo("<lang en>c</lang> <ruby>d<rt>e</rt></ruby> a<00:00:01.500>f <x>g</x>"), []string{p + `c d (e) af g</p>`}},
		{
			"resets from inherited styles",
			strings.Replace(styledDocument(`<head><styling><style xml:id="i" tts:fontStyle="italic" tts:fontWeight="bold" tts:textDecoration="underline lineThrough" tts:color="yellow"/></styling></head>`,
				"<div>"+oneParagraph+"</div>", ""), "<body>", `<body style="i">`, 1),
			linesTo(`<i>a</i> b <font color="red">c</font> <c.cyan>d</c> <b><u>e</u></b>`),
			[]string{p + `<span tts:fontWeight="normal" tts:fontStyle="normal" tts:textDecoration="noUnderline noLineThrough" tts:color="white">` +
				`<span tts:fontStyle="italic">a</span> b <span tts:color="red">c</span> <span tts:color="#00ffff">d</span> ` +
				`<span tts:fontWeight="bold"><span tts:textDecoration="underline">e</span></span></span></p>`},
		},
		{
			"no reset where the open spans repeat the style", styledDocument("<head/>", `<div tts:fontStyle="italic">`+oneParagraph+"</div>", ""), linesTo("<i>a</i> <i> </i>"),
			[]string{p + `<span tts:fontStyle="italic">a</span> <span tts:fontStyle="italic"> </span></p>`},
		},
		{"text characters", in, linesTo("a\x01b &amp; c &lt;", "d"), []string{p + `ab &amp; c &lt;<br/>d</p>`}},
		{"entities", in, linesTo(`&nbsp;&eacute;&#65;&amp;&lt;b&gt;&quot;'"`), []string{p + "\u00a0\u00e9A&amp;&lt;b&gt;\"'\"</p>"}},
		{"title with forbidden characters", in, titleTo("a\x01<b>&\uffff"), []string{"<ttm:title>a&lt;b&gt;&amp;</ttm:title>"}},
	})
}

func TestStoredAttributes(t *testing.T) {
	t.Parallel()

	runWriteRules(t, []writeRule{
		{
			"dropped root parameters", `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttp="http://www.w3.org/ns/ttml#parameter" xmlns:p="http://www.w3.org/2006/10/ttaf1#parameter"` +
				` ttp:timeBase="media" ttp:frameRate="25" ttp:tickRate="10" p:clockMode="utc" ttp:markerMode="discontinuous" timeBase="x" foo:dropMode="a" ttp:dropMode="nonDrop">` +
				"<body><div>" + oneParagraph + "</div></body></tt>", nil,
			[]string{` xml:lang="" ttp:frameRate="25" ttp:tickRate="10" timeBase="x">`},
		},
		{
			"undeclared and inner prefixes are lost", styledDocument("<head/>",
				`<div xmlns:q="urn:q" q:a="1" b="2&quot;&lt;&amp;"><p begin="1s" end="2s" xmlns:z="urn:z" z:a="1" tts:color="red" xml:space="preserve" style="s">x</p></div>`, ""), nil,
			[]string{`<div b="2&quot;&lt;&amp;">`, `<p begin="00:00:01.000" end="00:00:02.000" tts:color="red" xml:space="preserve" style="s"><span tts:color="#ff0000">x</span></p>`},
		},
		{
			"consecutive cues with the same div share it",
			styledDocument("<head/>", `<div b="1"><p begin="1s" end="2s">x</p><p begin="3s" end="4s">y</p></div><div b="1"><p begin="5s" end="6s">z</p></div>`+
				`<div b="2"><p begin="7s" end="8s">w</p></div><div><div b="1"><p begin="9s" end="10s">v</p></div></div>`, ""), nil,
			[]string{"    <div b=\"1\">\n      <p begin=\"00:00:01.000\" end=\"00:00:02.000\">x</p>\n      <p begin=\"00:00:03.000\" end=\"00:00:04.000\">y</p>\n" +
				"      <p begin=\"00:00:05.000\" end=\"00:00:06.000\">z</p>\n    </div>\n    <div b=\"2\">\n      <p begin=\"00:00:07.000\" end=\"00:00:08.000\">w</p>\n" +
				"    </div>\n    <div b=\"1\">\n      <p begin=\"00:00:09.000\" end=\"00:00:10.000\">v</p>\n    </div>\n"},
		},
		{
			"body attributes", strings.Replace(styledDocument("<head/>", "<div>"+oneParagraph+"</div>", ""), "<body>", `<body begin="0s" tts:color="red" xml:id="bd" region="r">`, 1), nil,
			[]string{`<body tts:color="red" xml:id="bd" region="r">`, `<p begin="00:00:01.000" end="00:00:02.000" region="r"><span tts:color="#ff0000">x</span></p>`},
		},
		{
			"agents of the head", styledDocument(`<head><metadata><ttm:agent xml:id="a1"><ttm:name> Ann </ttm:name></ttm:agent>`+
				`<ttm:agent xml:id="a2"><ttm:name>Ann</ttm:name><ttm:name>Bo</ttm:name></ttm:agent></metadata></head>`, "<div>"+oneParagraph+"</div>", ""),
			linesTo("<v Bo>a <v Cy>b <v Ann>c"),
			[]string{
				`</metadata>` + "\n    " + `<ttm:agent xml:id="agent3" type="person"><ttm:name type="full">Cy</ttm:name></ttm:agent>`,
				`<p begin="00:00:01.000" end="00:00:02.000"><span ttm:agent="a2">a </span><span ttm:agent="agent3">b </span><span ttm:agent="a1">c</span></p>`,
			},
		},
	})
}

// Covers https://github.com/yama6a/subtitle-toolbox-php/issues/1071, which Go does not copy.
func TestAttributeValuesKeepTabLFAndCR(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, strings.Replace(styledDocument("<head/>", `<div><p begin="1s" end="2s" foo="a&#13;b">x</p></div>`, ""), "<body>", `<body baz="b&#13;c">`, 1), false)
	out := writeDefault(t, s)
	assert.Contains(t, out, `<body baz="b&#13;c">`)
	assert.Contains(t, out, `<p begin="00:00:01.000" end="00:00:02.000" foo="a&#13;b">x</p>`)
	back, _ := mustRead(t, out, false)
	assert.Equal(t, out, writeDefault(t, back))

	require.NoError(t, s.SetFormatData("ttml", model.NewData(
		model.Field{Key: "attributes", Value: attrs("baz", "1\t2")},
		model.Field{Key: "body", Value: attrs("qux", "d\ne")},
	)))
	out = writeDefault(t, s)
	assert.Contains(t, out, ` xml:lang="" baz="1&#9;2">`)
	assert.Contains(t, out, `<body qux="d&#10;e">`)
}

func TestOutputOptions(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, styledDocument("<head>\n<styling/>\n</head>", "<div>"+oneParagraph+"</div>", ""), false)
	crlf := writeTTML(t, s, model.CommonWriteOptions{LineEnding: model.CRLF})
	assert.Equal(t, strings.ReplaceAll(writeDefault(t, s), "\n", "\r\n"), crlf)
	assert.Contains(t, crlf, "<head>\r\n<styling/>\r\n</head>")
	assert.True(t, strings.HasPrefix(writeTTML(t, s, model.CommonWriteOptions{BOM: model.BOMAdd}), "\ufeff<?xml"))
	assert.True(t, strings.HasPrefix(writeDefault(t, s), "<?xml"))

	stripped := writeTTML(t, readSRT(t, "1\n00:00:01,000 --> 00:00:02,000\n<v Ann><b>a</b> b\nc\n"), model.CommonWriteOptions{StripTags: true})
	assert.Contains(t, stripped, `<p begin="00:00:01.000" end="00:00:02.000" region="bottomCenter">a b<br/>c</p>`)
	assert.NotContains(t, stripped, "agent")
}

func TestTimes(t *testing.T) {
	t.Parallel()

	c := cue(-5*time.Second, 1500*time.Millisecond, "x")
	c.Lines = []string{"<00:00:01.000>x"}
	out := writeDefault(t, subtitleOf(c, cue(100*time.Hour, 100*time.Hour+time.Millisecond, "y")))
	assert.Contains(t, out, `<p begin="00:00:00.000" end="00:00:01.500" region="bottomCenter">x</p>`)
	assert.Contains(t, out, `<p begin="100:00:00.000" end="100:00:00.001" region="bottomCenter">y</p>`)
}

func TestStoredHeadThatIsNoHead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, head string
		syntax     bool
	}{
		{"not XML", "<head><styling></head>", true},
		{"empty", "", false},
		{"another element", "<body/>", false},
		{"text first", " <head/>", false},
		{"undeclared prefix", "<x:head/>", false},
		{"not UTF-8", "<head>\xe9</head>", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleOf(cue(0, time.Second, "x"))
			require.NoError(t, s.SetFormatData("ttml", model.NewData(model.Field{Key: "head", Value: tt.head})))
			uerr := writeError(t, s)
			assert.Equal(t, "The stored TTML head is not a well-formed <head> element.", uerr.Reason)
			assert.Equal(t, -1, uerr.CueIndex)
			assert.Equal(t, tt.syntax, uerr.Err != nil)
		})
	}
}

func TestStoredNamespacesAsListOrObject(t *testing.T) {
	t.Parallel()

	for _, namespaces := range []any{[]any{}, model.NewData(), attrs("x", "urn:x")} {
		s := subtitleOf(cue(0, time.Second, "x"))
		require.NoError(t, s.SetFormatData("ttml", model.NewData(
			model.Field{Key: "namespace", Value: "http://www.w3.org/2006/10/ttaf1"},
			model.Field{Key: "namespaces", Value: namespaces},
			model.Field{Key: "attributes", Value: []any{}},
		)))
		out := writeDefault(t, s)
		assert.Contains(t, out, `<tt xmlns="http://www.w3.org/2006/10/ttaf1" `)
		assert.Contains(t, out, `<p begin="00:00:00.000" end="00:00:01.000">x</p>`)
	}
}

var errWrite = errors.New("disk full")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestWriteReturnsTheWriterError(t *testing.T) {
	t.Parallel()

	err := ttml.Write(failingWriter{}, &model.Subtitle{}, model.CommonWriteOptions{}, ttml.WriteOptions{})
	require.ErrorIs(t, err, errWrite)
}

// The new elements take the prefixes and declarations that libxml gives them when PHP inserts them into the head.
func TestNewElementsInAHeadWithItsOwnDeclarations(t *testing.T) {
	t.Parallel()

	const (
		styling  = "http://www.w3.org/ns/ttml#styling"
		metadata = "http://www.w3.org/ns/ttml#metadata"
		title    = "\n    <ttm:title>T</ttm:title>"
		agent    = `<ttm:agent xml:id="agent1" type="person"><ttm:name type="full">Ann</ttm:name></ttm:agent>`
		region   = `xml:id="topLeft" tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="before" tts:textAlign="left"/>`
	)
	change := func(t *testing.T, s *model.Subtitle) {
		t.Helper()
		s.SetMetadata(model.MetadataTitle, "T")
		setCue(t, s, 0, func(c *model.Cue) { c.Alignment, c.Lines = model.TopLeft, []string{"<v Ann>x"} })
	}
	body := "<div>" + oneParagraph + "</div>"
	runWriteRules(t, []writeRule{
		{
			"prefixed root", `<tt:tt xmlns:tt="http://www.w3.org/ns/ttml"><tt:head/><tt:body><tt:div><tt:p begin="1s" end="2s">x</tt:p></tt:div></tt:body></tt:tt>`, change,
			[]string{"  <tt:head>" + title + "\n    " + agent + "\n    <tt:layout>\n      <tt:region " + region + "\n    </tt:layout>\n  </tt:head>\n"},
		},
		{
			"layout with its own prefix", styledDocument(`<head><tt:layout xmlns:tt="http://www.w3.org/ns/ttml"/></head>`, body, ""), change,
			[]string{"  <head>" + title + "\n    " + agent + `<tt:layout xmlns:tt="http://www.w3.org/ns/ttml">` + "\n      <tt:region " + region + "\n    </tt:layout></head>\n"},
		},
		{
			"head with other prefixes for the namespaces", styledDocument(`<head xmlns:m="`+metadata+`" xmlns:s="`+styling+`" xmlns:b="http://www.w3.org/ns/ttml"><styling/></head>`, body, ""), change,
			[]string{
				"\n    <ttm:title xmlns:ttm=\"" + metadata + "\">T</ttm:title>",
				`<ttm:agent xmlns:ttm="` + metadata + `" xml:id="agent1" type="person">`,
				"\n    <b:layout>\n      <b:region xmlns:tts=\"" + styling + "\" " + region + "\n    </b:layout>\n",
			},
		},
		{
			"head that binds ttm to another namespace", styledDocument(`<head xmlns:ttm="urn:m"><styling/></head>`, body, ""), change,
			[]string{"\n    <ttm:title xmlns:ttm=\"" + metadata + "\">T</ttm:title>", `<ttm:agent xmlns:ttm="` + metadata + `" xml:id="agent1" type="person">`},
		},
		{
			"head that binds tts to another namespace", styledDocument(`<head xmlns:tts="urn:t"><layout/></head>`, body, ""), change,
			[]string{"<layout>\n      <region xmlns:tts=\"" + styling + "\" " + region + "\n    </layout></head>\n"},
		},
		{
			"layout with another prefix for styling", styledDocument(`<head><layout xmlns:s="`+styling+`"/></head>`, body, ""), change,
			[]string{`<layout xmlns:s="` + styling + `">` + "\n      <region xmlns:tts=\"" + styling + "\" " + region},
		},
		{
			"other declarations deeper in the head", styledDocument(`<head><metadata xmlns:ttm="urn:z"><ttm:x/></metadata><styling xmlns="urn:q"/></head>`, body, ""), change,
			[]string{"  <head>" + title + `<metadata xmlns:ttm="urn:z"><ttm:x/></metadata>` + "\n    " + agent + `<styling xmlns="urn:q"/>` + "\n    <layout>\n      <region " + region},
		},
	})
}
