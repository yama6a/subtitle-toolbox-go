package ttml_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// The tests in this file port tests/Parsers/TtmlParserTest.php of subtitle-toolbox-php, then add rows of their own.

func TestReadsBeginEndDurItalicSpanAndLanguage(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, "<tt xmlns=\"http://www.w3.org/ns/ttml\" xmlns:tts=\"http://www.w3.org/ns/ttml#styling\" xml:lang=\"en\">\n"+
		"  <body>\n    <div>\n"+
		"      <p begin=\"00:00:01.500\" end=\"00:00:04.000\">Hello<br/><span tts:fontStyle=\"italic\">world</span></p>\n"+
		"      <p begin=\"5s\" dur=\"2500ms\">Second cue</p>\n"+
		"    </div>\n  </body>\n</tt>", false)

	lang, _ := s.Metadata(model.MetadataLanguage)
	assert.Equal(t, "en", lang)
	assert.Empty(t, warnings)
	c0, _ := s.Cue(0)
	c1, _ := s.Cue(1)
	assert.Equal(t, []string{"Hello", "<i>world</i>"}, c0.Lines)
	assert.Equal(t, []cueRow{{sec(1.5), sec(4), "Hello\n<i>world</i>"}, {sec(5), sec(7.5), "Second cue"}}, cueRows(s))
	assert.Equal(t, []string{"Second cue"}, c1.Lines)
}

func TestTimeExpressions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, expression, root string
		want                   float64
	}{
		{"clock time", "00:00:01.500", "", 1.5},
		{"clock time, long hours", "100:00:00", "", 360000},
		{"clock time with frames", "00:00:01:12", `ttp:frameRate="24"`, 1.5},
		{"frames, default 30 fps", "00:00:01:15", "", 1.5},
		{"sub-frames", "00:00:00:01.1", `ttp:frameRate="25" ttp:subFrameRate="2"`, 0.06},
		{"frame rate multiplier", "24f", `ttp:frameRate="24" ttp:frameRateMultiplier="1000 1001"`, 1.001},
		{"hours", "1.5h", "", 5400},
		{"minutes", "2m", "", 120},
		{"seconds", "1.5s", "", 1.5},
		{"milliseconds", "1500ms", "", 1.5},
		{"frames", "36f", `ttp:frameRate="24"`, 1.5},
		{"ticks", "15000000t", `ttp:tickRate="10000000"`, 1.5},
		{"ticks at the frame rate", "36t", `ttp:frameRate="24"`, 1.5},
		{"ticks, default rate 1", "2t", "", 2},
		{"comma decimal separator", "00:00:01,500", "", 1.5},
		{"fraction wins over frames", "00:00:01.5", `ttp:frameRate="24"`, 1.5},
		{"minutes and seconds not range-checked", "00:99:99", "", 6039},
		{"hundredths of a frame rate below 1", "1f", `ttp:frameRate="0.5"`, 2},
		{"multiplier with a colon", "30f", `ttp:frameRate="30" ttp:frameRateMultiplier="1000:1001"`, 1.001},
		{"bad multiplier gives 1 1", "30f", `ttp:frameRate="30" ttp:frameRateMultiplier="1000 0"`, 1},
		{"frame rate 0 gives 30", "30f", `ttp:frameRate="0"`, 1},
		{"tick rate 0 gives the frame rate", "25t", `ttp:frameRate="25" ttp:tickRate="0"`, 1},
		{"sub-frame rate 0 gives 1", "00:00:00:01.1", `ttp:frameRate="25" ttp:subFrameRate="0"`, 0.08},
		{"frame rate as PHP reads a float", "25f", `ttp:frameRate=" 25fps"`, 1},
		{"white space around the expression", " 1.5s ", "", 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := paragraphCue(t, `<p begin="`+tt.expression+`" end="999999s">x</p>`, tt.root, "")
			assert.Equal(t, sec(tt.want), c.Start)
		})
	}
}

func TestSMPTETimeExpressions(t *testing.T) {
	t.Parallel()

	ntsc := `ttp:timeBase="smpte" ttp:frameRate="30" ttp:frameRateMultiplier="1000 1001"`
	tests := []struct {
		name, expression, root string
		want                   float64
	}{
		{"29.97 fps non-drop, one hour", "01:00:00:00", ntsc + ` ttp:dropMode="nonDrop"`, 3603.6},
		{"29.97 fps, non-drop is the default", "01:00:00:00", ntsc, 3603.6},
		{"29.97 fps drop, after minute 1", "00:01:00:02", ntsc + ` ttp:dropMode="dropNTSC"`, 60.06},
		{"29.97 fps drop, last frame before", "00:00:59:29", ntsc + ` ttp:dropMode="dropNTSC"`, 60.027},
		{"29.97 fps drop, minute 10", "00:10:00:00", ntsc + ` ttp:dropMode="dropNTSC"`, 599.999},
		{"29.97 fps drop, one hour", "01:00:00:00", ntsc + ` ttp:dropMode="dropNTSC"`, 3599.996},
		{
			"Apple 999/1000 drop, minute 10", "00:10:00:00",
			`ttp:timeBase="smpte" ttp:frameRate="30" ttp:frameRateMultiplier="999 1000" ttp:dropMode="dropNTSC"`, 600,
		},
		{"drop PAL, minute 10", "01:10:00:04", ntsc + ` ttp:dropMode="dropPAL"`, 4200.063},
		{"23.976 fps", "00:00:01:00", `ttp:timeBase="smpte" ttp:frameRate="24" ttp:frameRateMultiplier="1000 1001"`, 1.001},
		{"25 fps, one hour", "01:00:00:00", `ttp:timeBase="smpte" ttp:frameRate="25"`, 3600},
		{"25 fps, frames", "00:00:01:05", `ttp:timeBase="smpte" ttp:frameRate="25" ttp:dropMode="dropNTSC"`, 1.2},
		{"sub-frames", "00:00:00:01.1", `ttp:timeBase="smpte" ttp:frameRate="25" ttp:subFrameRate="2"`, 0.06},
		{"media time base, frames", "01:00:00:00", `ttp:frameRate="30" ttp:frameRateMultiplier="1000 1001"`, 3600},
		{"media time base, drop mode ignored", "00:01:00:02", `ttp:timeBase="media" ttp:frameRate="30" ttp:dropMode="dropNTSC"`, 60.067},
		{"drop mode also for a time without frames", "00:10:00", ntsc + ` ttp:dropMode="dropNTSC"`, 599.999},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := paragraphCue(t, `<p begin="`+tt.expression+`" end="99:00:00:00">x</p>`, tt.root, "")
			assert.Equal(t, sec(tt.want), c.Start)
		})
	}
}

func TestInvalidSpanTimeKeepsTheText(t *testing.T) {
	t.Parallel()

	c := paragraphCue(t, `<p begin="1s" end="2s">A <span begin="x">B</span></p>`, "", "")
	assert.Equal(t, cueRow{sec(1), sec(2), "A B"}, cueRow{c.Start, c.End, c.Text()})
}

func TestInvalidTimeExpression(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, expression string }{
		{"bare seconds", "1.5"},
		{"one-digit fields", "1:2:3"},
		{"unknown unit", "5d"},
		{"empty", ""},
		{"sign", "-1s"},
		{"frames of 3 digits", "00:00:01:500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perr := readError(t, document(`<div><p begin="`+tt.expression+`" end="2s">x</p></div>`, "", ""), false)
			assert.Equal(t, `The time expression "`+tt.expression+`" is not valid.`, perr.Msg)
			assert.Equal(t, model.NoLine, perr.Line)
		})
	}
}

func TestTimeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, p, want string }{
		{"expression alone", `<p begin="100000:00:00" end="1s">x</p>`, `The time "100000:00:00" is not below 100000 hours.`},
		{"expression that rounds up", `<p begin="99999:59:59.9999" end="1s">x</p>`, `The time "99999:59:59.9999" is not below 100000 hours.`},
		{"offset in hours", `<p begin="1s" end="100000h">x</p>`, `The time "100000h" is not below 100000 hours.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, readError(t, document("<div>"+tt.p+"</div>", "", ""), false).Msg)
		})
	}

	perr := readError(t, document(`<div begin="99999h">`+"\n"+`<p begin="3600s" end="3601s">x</p></div>`, "", ""), false)
	assert.Equal(t, `The time "3600s" is not below 100000 hours.`, perr.Msg, "the sum with the parent offset")
	assert.Equal(t, 2, perr.Line)
	c := paragraphCue(t, `<p begin="99999:59:59.9994" end="99999:59:59.9994">x</p>`, "", "")
	assert.Equal(t, sec(359999999.999), c.Start)
}

func TestParentBeginOffsetsAndEnds(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, document(`<div begin="10s" end="20s"><div begin="1s">`+
		`<p begin="1s" end="2s">relative</p>`+
		`<p begin="2s">parent end</p>`+
		`<p begin="3s" end="30s">clipped</p>`+
		`<p begin="1s" end="5s" dur="1s">earlier of end and dur</p>`+
		`</div></div>`, "", ""), false)

	assert.Equal(t, []cueRow{
		{sec(12), sec(13), "relative"},
		{sec(12), sec(13), "earlier of end and dur"},
		{sec(13), sec(20), "parent end"},
		{sec(14), sec(20), "clipped"},
	}, cueRows(s))
}

func TestNestedTimedDivs(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, document(`<div begin="10s" dur="30s"><div begin="5s" end="8s"><p begin="1s" end="20s">inner end</p></div>`+
		`<div begin="2s"><p dur="1s">dur from the div begin</p><p end="50s">outer end</p></div></div>`, "", ""), false)

	assert.Equal(t, []cueRow{
		{sec(12), sec(13), "dur from the div begin"},
		{sec(12), sec(40), "outer end"},
		{sec(16), sec(18), "inner end"},
	}, cueRows(s))
}

func TestBodyBeginOffset(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, header+`><body begin="5s"><div><p begin="1s" end="2s">x</p></div></body></tt>`, false)
	assert.Equal(t, []cueRow{{sec(6), sec(7), "x"}}, cueRows(s))
}

func TestParagraphWithoutEnd(t *testing.T) {
	t.Parallel()

	perr := readError(t, document("<div>\n"+`<p begin="1.25s">x</p></div>`, "", ""), false)
	assert.Equal(t, "The paragraph that begins at 1.25s has no end time.", perr.Msg)
	assert.Equal(t, 2, perr.Line)
}

// Rows of the Decision on OrderedTimes in https://github.com/yama6a/subtitle-toolbox-go/issues/23.
func TestParagraphEndBeforeBegin(t *testing.T) {
	t.Parallel()

	content := document("<div>\n"+`<p begin="5s" end="2s">x</p>`+"\n"+`<p begin="40.001s" end="10s">y</p></div>`, "", "")
	perr := readError(t, content, false)
	assert.Equal(t, "The cue ends at 2 s, before it starts at 5 s.", perr.Msg)
	assert.Equal(t, 2, perr.Line)

	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{{sec(2), sec(5), "x"}}, cueRows(s))
	assert.Equal(t, []warningRow{
		{2, 0, model.Repaired, "The cue ends at 2 s, before it starts at 5 s. The parser swapped the times."},
		{3, 1, model.Skipped, "The cue ends at 10 s, before it starts at 40.001 s."},
	}, warningRows(warnings))
	assert.Equal(t, []string{`<p begin="5s" end="2s">x</p>`}, warnings[0].Lines)
}

func TestWhiteSpaceIsCollapsed(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, p, want string }{
		{
			"PHP row",
			"<p end=\"1s\">\n  Hello  <span tts:fontWeight=\"bold\"> bold </span>\n  world <br/>  next\tline\n</p>",
			"Hello <b>bold </b>world\nnext line",
		},
		{"CR and CR LF", "<p end=\"1s\">a&#13;&#10;b&#13;c</p>", "a b c"},
		{"preserve turns LF into a line break", "<p xml:space=\"preserve\" end=\"1s\">first\nsecond</p>", "first\nsecond"},
		{"preserve still collapses spaces", "<p xml:space=\"preserve\" end=\"1s\">a   b\n  c</p>", "a b\nc"},
		{"preserve from the div", "<p end=\"1s\"><span xml:space=\"preserve\">a\nb</span> c</p>", "a\nb c"},
		{"default inside preserve", "<p xml:space=\"preserve\" end=\"1s\"><span xml:space=\"default\">a\nb</span></p>", "a b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, paragraphCue(t, tt.p, "", "").Text())
		})
	}
}

func TestTextEntitiesStayEscaped(t *testing.T) {
	t.Parallel()

	assert.Equal(t, `a &lt;b&gt; &amp; "c"`, paragraphCue(t, `<p end="1s">a &lt;b&gt; &amp; "c"</p>`, "", "").Text())
}

func TestOtherElementsAreSkippedWithTheirText(t *testing.T) {
	t.Parallel()

	p := `<p end="1s">a<set tts:color="red"/><metadata>b</metadata><x:y xmlns:x="urn:x">c</x:y><!-- d --> e</p>`
	assert.Equal(t, "a e", paragraphCue(t, p, "", "").Text())
}

func TestStylesMapToCoreMarkup(t *testing.T) {
	t.Parallel()

	head := `<styling>` +
		`<style xml:id="base" tts:fontWeight="bold"/>` +
		`<style xml:id="chained" style="base" tts:color="yellow"/>` +
		`<style xml:id="italic" tts:fontStyle="oblique"/>` +
		`</styling>`
	text := paragraphCue(t, `<p end="1s" style="chained">a <span style="italic" tts:fontWeight="normal">b</span>`+
		` <span tts:textDecoration="underline lineThrough">c</span>`+
		` <span tts:textDecoration="underline"><span tts:textDecoration="noUnderline">d</span></span>`+
		` <span tts:color="rgb(0, 128, 255)">e</span> <span tts:color="#00FF0080">f</span>`+
		` <span tts:color="transparent">g</span></p>`, "", head).Text()

	assert.Equal(t, `<font color="#ffff00"><b>a </b><i>b </i><b><u><s>c </s></u>d </b></font>`+
		`<font color="#0080ff"><b>e </b></font><font color="#00ff00"><b>f </b></font><b>g</b>`, text)
}

func TestStyleProperties(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, span, want string }{
		{"bold", `tts:fontWeight="bold"`, "<b>x</b>"},
		{"bold with spaces", `tts:fontWeight=" bold "`, "<b>x</b>"},
		{"italic", `tts:fontStyle="italic"`, "<i>x</i>"},
		{"oblique", `tts:fontStyle="oblique"`, "<i>x</i>"},
		{"underline", `tts:textDecoration="underline"`, "<u>x</u>"},
		{"line-through", `tts:textDecoration="lineThrough"`, "<s>x</s>"},
		{"none, then underline", `tts:textDecoration="none underline"`, "<u>x</u>"},
		{"hex color", `tts:color="#FF0000"`, `<font color="#ff0000">x</font>`},
		{"hex color with alpha", `tts:color="#ff000080"`, `<font color="#ff0000">x</font>`},
		{"rgba color", `tts:color="rgba(255,0,0,128)"`, `<font color="#ff0000">x</font>`},
		{"channel above 255", `tts:color="rgb(300,0,0)"`, `<font color="#ff0000">x</font>`},
		{"named color in mixed case", `tts:color="Lime"`, `<font color="#00ff00">x</font>`},
		{"cyan", `tts:color="cyan"`, `<font color="#00ffff">x</font>`},
		{"white gives no tag", `tts:color="white"`, "x"},
		{"unknown color", `tts:color="orange"`, "x"},
		{"3 digit hex", `tts:color="#f00"`, "x"},
		{"prefix bound to no namespace", `xmlns:tts="" tts:fontWeight="bold"`, "<b>x</b>"},
		{"background color is not read", `tts:backgroundColor="red"`, "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, paragraphCue(t, `<p end="1s"><span `+tt.span+`>x</span></p>`, "", "").Text())
		})
	}
}

func TestStylingAttributeWithUndeclaredPrefix(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, `<tt xmlns="http://www.w3.org/ns/ttml"><body><div><p end="1s" tts:fontStyle="italic">x</p></div></body></tt>`, false)
	c, _ := s.Cue(0)
	assert.Equal(t, "<i>x</i>", c.Text())
}

func TestDFXPStylingNamespaces(t *testing.T) {
	t.Parallel()

	for _, ns := range []string{
		"http://www.w3.org/2006/10/ttaf1#style", "http://www.w3.org/2006/10/ttaf1#styling",
		"http://www.w3.org/2006/04/ttaf1#styling", "http://www.w3.org/2006/04/ttaf1#style", "urn:other",
	} {
		s, _ := mustRead(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:s="`+ns+`"><body><div><p end="1s" s:fontWeight="bold">x</p></div></body></tt>`, false)
		c, _ := s.Cue(0)
		want := "<b>x</b>"
		if ns == "urn:other" {
			want = "x"
		}
		assert.Equal(t, want, c.Text(), ns)
	}
}

func TestStyleReferencesInALoop(t *testing.T) {
	t.Parallel()

	head := `<styling><style xml:id="a" style="b" tts:fontWeight="bold"/><style xml:id="b" style="a" tts:fontStyle="italic"/></styling>`
	assert.Equal(t, "<b><i>x</i></b>", paragraphCue(t, `<p end="1s" style="a">x</p>`, "", head).Text())
}

func TestFirstStyleOfAnIDWins(t *testing.T) {
	t.Parallel()

	head := `<styling><style xml:id="a" tts:fontWeight="bold"/><style xml:id="a" tts:fontStyle="italic"/></styling>`
	assert.Equal(t, "<b>x</b>", paragraphCue(t, `<p end="1s" style="a">x</p>`, "", head).Text())
}

func TestRegionStylesInThreePlaces(t *testing.T) {
	t.Parallel()

	head := `<styling><style xml:id="s" tts:fontWeight="bold" tts:color="red"/></styling><layout>` +
		`<region xml:id="r" style="s" tts:color="lime"><style tts:fontStyle="italic" tts:color="yellow"/></region></layout>`
	c := paragraphCue(t, `<p end="1s" region="r">x</p>`, "", head)
	assert.Equal(t, `<font color="#00ff00"><b><i>x</i></b></font>`, c.Text(), "the own attributes of the region win over its nested style")
}

func TestStyleInheritanceOrder(t *testing.T) {
	t.Parallel()

	head := `<layout><region xml:id="r" tts:color="red" tts:fontWeight="bold"/></layout>`
	s, _ := mustRead(t, document(`<div tts:color="yellow"><p end="1s" tts:fontWeight="normal">a<span tts:color="white">b</span></p></div>`,
		"", head), false)
	c, _ := s.Cue(0)
	assert.Equal(t, `<font color="#ffff00">a</font>b`, c.Text())
}

func TestWhiteTextGivesNoFontTag(t *testing.T) {
	t.Parallel()

	head := `<styling><style xml:id="s1" tts:color="white" tts:fontSize="100%"/></styling>`
	text := paragraphCue(t, `<p end="1s" style="s1">white <span tts:color="yellow">yellow</span>`+
		` <span tts:color="rgb(255,255,255)">rgb</span> <span tts:color="rgba(255,255,255,255)">rgba</span>`+
		` <span tts:color="#FFFFFFFF">hex</span></p>`, "", head).Text()

	assert.Equal(t, `white <font color="#ffff00">yellow </font>rgb rgba hex`, text)
}

func TestAgentsBecomeSpeakerTags(t *testing.T) {
	t.Parallel()

	head := `<metadata><ttm:agent xml:id="a1" type="person"><ttm:name type="full">Anna</ttm:name></ttm:agent></metadata>`
	text := paragraphCue(t, `<p end="1s" ttm:agent="a1">Hi<br/>there <span ttm:agent="b2">Bye</span></p>`, "", head).Text()

	assert.Equal(t, "<v Anna>Hi\nthere </v><v b2>Bye", text)
}

func TestAgentNames(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, head, p, want string }{
		{
			"first name with text, trimmed",
			`<ttm:agent xml:id="a"><ttm:name> </ttm:name><ttm:name> Anna </ttm:name><ttm:name>Ann</ttm:name></ttm:agent>`,
			`<p end="1s" ttm:agent="a">Hi</p>`, "<v Anna>Hi",
		},
		{
			"a later agent of the same ID wins",
			`<ttm:agent xml:id="a"><ttm:name>Anna</ttm:name></ttm:agent><styling><ttm:agent xml:id="a"><ttm:name>Ben</ttm:name></ttm:agent></styling>`,
			`<p end="1s" ttm:agent="a">Hi</p>`, "<v Ben>Hi",
		},
		{"the first of several IDs", "", `<p end="1s" ttm:agent=" b  c ">Hi</p>`, "<v b>Hi"},
		{"no voice for white space", "", `<p end="1s"> <span ttm:agent="a">Hi</span></p>`, "<v a>Hi"},
		{"name escaped", `<ttm:agent xml:id="a"><ttm:name>A &amp; B</ttm:name></ttm:agent>`, `<p end="1s" ttm:agent="a">Hi</p>`, "<v A &amp; B>Hi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, paragraphCue(t, tt.p, "", tt.head).Text())
		})
	}
}

func TestAgentAttributeIsNotStored(t *testing.T) {
	t.Parallel()

	c := paragraphCue(t, `<p end="1s" ttm:agent="a" xml:id="c1" tts:color="red">Hi</p>`, "", "")
	assert.Equal(t, "c1", c.ID)
	assert.Equal(t, attrs("tts:color", "red"), objectOf(t, cueData(t, c), "attributes"))
}

func TestRegionMapsToAlignment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, region string
		want         model.Alignment
	}{
		{"bottom center", `tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="after" tts:textAlign="center"`, model.BottomCenter},
		{"top left", `tts:displayAlign="before" tts:textAlign="left"`, model.TopLeft},
		{"middle right", `tts:displayAlign="center" tts:textAlign="right"`, model.MiddleRight},
		{"small region at the bottom", `tts:origin="10% 80%" tts:extent="80% 10%" tts:displayAlign="before" tts:textAlign="center"`, model.BottomCenter},
		{"pixels", `tts:origin="0px 400px" tts:extent="640px 80px" tts:textAlign="center"`, model.BottomCenter},
		{"cells", `tts:origin="0c 13c" tts:extent="32c 2c" tts:textAlign="center"`, model.BottomCenter},
		{"start depends on direction", `tts:displayAlign="after" tts:textAlign="start"`, model.BottomCenter},
		{"no text alignment", `tts:displayAlign="after"`, model.BottomCenter},
		{"unknown unit", `tts:origin="10em 10em" tts:textAlign="center"`, model.BottomCenter},
		{"auto origin and extent", `tts:origin="auto" tts:extent="auto" tts:displayAlign="center" tts:textAlign="left"`, model.MiddleLeft},
		{"middle row at the edge", `tts:origin="0% 50%" tts:extent="10% 1%" tts:textAlign="right"`, model.MiddleRight},
		{"unknown display alignment", `tts:displayAlign="middle" tts:textAlign="center"`, model.BottomCenter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, document(`<div><p region="r1" end="1s">x</p></div>`, `tts:extent="640px 480px"`,
				`<layout><region xml:id="r1" `+tt.region+`/></layout>`), false)
			c, _ := s.Cue(0)
			assert.Equal(t, tt.want, c.Alignment)
		})
	}
}

func TestAlignmentRules(t *testing.T) {
	t.Parallel()

	region := `<layout><region xml:id="r" tts:origin="0c 1c" tts:extent="32c 2c"/></layout>`
	tests := []struct {
		name, root, body, head string
		want                   model.Alignment
		wantData               bool
	}{
		{"no regions: a top region from 0 to 1", "", `<div><p tts:textAlign="center" end="1s">x</p></div>`, "", model.TopCenter, true},
		{"no regions, no text alignment", "", `<div><p end="1s">x</p></div>`, "", model.BottomCenter, false},
		{"a paragraph without region in a file with regions", "", `<div><p tts:textAlign="left" end="1s">x</p></div>`, region, model.BottomCenter, false},
		{"an unknown region", "", `<div><p region="missing" tts:textAlign="center" end="1s">x</p></div>`, "", model.BottomCenter, false},
		{"cells with the cell resolution", `ttp:cellResolution="32 3"`, `<div><p region="r" tts:textAlign="left" end="1s">x</p></div>`, region, model.MiddleLeft, true},
		{"cells with a bad cell resolution", `ttp:cellResolution="32"`, `<div><p region="r" tts:textAlign="left" end="1s">x</p></div>`, region, model.BottomCenter, false},
		{
			"pixels with the extent of the body", "",
			`<div><p region="r" tts:textAlign="right" end="1s">x</p></div>`,
			`<layout><region xml:id="r" tts:origin="0px 0px" tts:extent="10px 10px"/></layout>`, model.BottomCenter, false,
		},
		{"text alignment of the div", "", `<div region="r" tts:textAlign="right"><p end="1s">x</p></div>`, region, model.TopRight, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, document(tt.body, tt.root, tt.head), false)
			c, _ := s.Cue(0)
			assert.Equal(t, tt.want, c.Alignment)
			d, _ := c.FormatData("ttml")
			_, ok := d.Get("alignment")
			assert.Equal(t, tt.wantData, ok)
		})
	}

	s, _ := mustRead(t, header+`><head><layout><region xml:id="r" tts:origin="0px 400px" tts:extent="640px 80px"/></layout></head>`+
		`<body tts:extent="640px 480px"><div><p region="r" tts:textAlign="center" end="1s">x</p></div></body></tt>`, false)
	c, _ := s.Cue(0)
	assert.Equal(t, model.BottomCenter, c.Alignment)
	assert.Equal(t, int64(2), intOf(t, cueData(t, c), "alignment"), "pixels with the extent of the body")
}

func TestParagraphTextAlignWinsOverRegion(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, document(`<div region="r1"><p tts:textAlign="left" end="1s">x</p></div>`, "",
		`<layout><region xml:id="r1"><style tts:displayAlign="after"/><style tts:textAlign="center"/></region></layout>`), false)
	c, _ := s.Cue(0)

	assert.Equal(t, model.BottomLeft, c.Alignment)
	assert.Equal(t, attrs("tts:textAlign", "left", "region", "r1"), objectOf(t, cueData(t, c), "attributes"))
}

func TestTitleAndHeadGoToMetadataAndFormatData(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, document(`<div><p end="1s">x</p></div>`, `xml:lang="de" ttp:frameRate="25"`,
		`<metadata><ttm:title>Wetter</ttm:title><ttm:copyright>Sample</ttm:copyright></metadata>`), false)
	d := fileData(t, s)

	assert.Equal(t, map[string]string{"language": "de", "title": "Wetter"}, s.AllMetadata())
	assert.Equal(t, "http://www.w3.org/ns/ttml", stringOf(t, d, "namespace"))
	assert.Equal(t, attrs("ttp:frameRate", "25"), objectOf(t, d, "attributes"))
	assert.Equal(t, "<head><metadata><ttm:copyright>Sample</ttm:copyright></metadata></head>", stringOf(t, d, "head"))
	assert.Equal(t, attrs("", "http://www.w3.org/ns/ttml", "ttm", "http://www.w3.org/ns/ttml#metadata",
		"ttp", "http://www.w3.org/ns/ttml#parameter", "tts", "http://www.w3.org/ns/ttml#styling"), objectOf(t, d, "namespaces"))
}

func TestTitleRemoval(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, head, wantHead, wantTitle string }{
		{
			"title and the white space before it",
			"\n  <metadata>\n    <ttm:title> T </ttm:title>\n    <ttm:desc>D</ttm:desc>\n  </metadata>\n",
			"<head>\n  <metadata>\n    <ttm:desc>D</ttm:desc>\n  </metadata>\n</head>", "T",
		},
		{"only the first title", "<ttm:title>A</ttm:title><ttm:title>B</ttm:title>", "<head><ttm:title>B</ttm:title></head>", "A"},
		{"empty title", "<ttm:title> </ttm:title>", "<head/>", ""},
		{"title of an undeclared prefix", `<x xmlns:ttm=""/><ttm:title>U</ttm:title>`, `<head><x xmlns:ttm=""/></head>`, "U"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, document(`<div><p end="1s">x</p></div>`, "", tt.head), false)
			assert.Equal(t, tt.wantHead, stringOf(t, fileData(t, s), "head"))
			title, _ := s.Metadata(model.MetadataTitle)
			assert.Equal(t, tt.wantTitle, title)
		})
	}
}

func TestHeadKeepsNonASCIIAttributeValuesAsUTF8(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, document(`<div><p end="1s">x</p></div>`, "",
		`<styling><style xml:id="s" tts:fontFamily="Caf&#233;" tts:color="`+"caf\u00e9"+`"/></styling>`), false)
	assert.Equal(t, `<head><styling><style xml:id="s" tts:fontFamily="`+"Caf\u00e9"+`" tts:color="`+"caf\u00e9"+`"/></styling></head>`,
		stringOf(t, fileData(t, s), "head"))
}

func TestFormatData(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:x="urn:x" xml:id="root" x:a="1" foo:b="2" xml:lang="en">`+
		`<body begin="0s" style="s" region="r"><div x:d="1" xml:id="d1" region="r2" begin="0s"><div x:d="2" x:e="3">`+
		`<p begin="0s" end="1s" xml:id="p1" ttm:agent="a">x</p></div></div></body></tt>`, false)
	d := fileData(t, s)
	assert.Equal(t, []string{"namespace", "namespaces", "attributes", "body"}, d.Keys())
	assert.Equal(t, attrs("xml:id", "root", "x:a", "1", "foo:b", "2"), objectOf(t, d, "attributes"))
	assert.Equal(t, attrs("style", "s", "region", "r"), objectOf(t, d, "body"))

	c, _ := s.Cue(0)
	cd := cueData(t, c)
	assert.Equal(t, []string{"attributes", "div"}, cd.Keys())
	assert.Equal(t, attrs("ttm:agent", "a", "region", "r2"), objectOf(t, cd, "attributes"), "an undeclared ttm:agent stays, the inherited region is added")
	assert.Equal(t, attrs("x:d", "2", "x:e", "3"), objectOf(t, cd, "div"))
}

func TestXMLNSWithAPrefixThatIsNoNCNameIsAnAttribute(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, `<tt xmlns:0="urn:x" a="1"><body><div><p end="1s">x</p></div></body></tt>`, false)
	d := fileData(t, s)
	assert.Equal(t, attrs("xmlns:0", "urn:x", "a", "1"), objectOf(t, d, "attributes"))
	v, _ := d.Get("namespaces")
	assert.Equal(t, []any{}, v)
}

func TestFormatDataWithoutNamespaceOrHead(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, `<tt><body><div><p end="1s">x</p></div></body></tt>`, false)
	d := fileData(t, s)
	assert.Equal(t, []string{"namespace", "namespaces", "attributes"}, d.Keys())
	assert.Empty(t, stringOf(t, d, "namespace"))
	c, _ := s.Cue(0)
	_, ok := c.FormatData("ttml")
	assert.False(t, ok)
}

func TestTTElementsCountOnlyInTheRootNamespace(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:x="urn:x"><body><div><x:p end="1s">a</x:p><foo:p end="1s">b</foo:p>`+
		`<p end="2s">c<x:br/>d<foo:br/>e<x:span>f</x:span></p></div></body></tt>`, false)
	assert.Equal(t, []cueRow{{0, sec(2), "cde"}}, cueRows(s))

	s, _ = mustRead(t, `<tt><body><div><p end="1s">a<br/>b</p><p xmlns="urn:x" end="1s">c</p></div></body></tt>`, false)
	assert.Equal(t, []cueRow{{0, sec(1), "a\nb"}}, cueRows(s))
}

func TestDFXPNamespacesAreRead(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, `<tt xmlns="http://www.w3.org/2006/10/ttaf1" xmlns:tts="http://www.w3.org/2006/10/ttaf1#style"`+
		` xmlns:ttp="http://www.w3.org/2006/10/ttaf1#parameter" ttp:frameRate="25">`+
		`<body><div><p begin="00:00:01:05" end="2s"><span tts:fontWeight="bold">x</span></p></div></body></tt>`, false)
	c, _ := s.Cue(0)
	assert.Equal(t, sec(1.2), c.Start)
	assert.Equal(t, "<b>x</b>", c.Text())
}

func Test2006NamespaceIsRead(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "<tt xmlns=\"http://www.w3.org/2006/04/ttaf1\" xmlns:tts=\"http://www.w3.org/2006/04/ttaf1#styling\" xml:lang=\"en\">\n"+
		`  <body><div><p begin="00:00:01.00" end="00:00:03.00" tts:fontStyle="italic">Hello</p></div></body></tt>`, false)
	assert.Equal(t, []cueRow{{sec(1), sec(3), "<i>Hello</i>"}}, cueRows(s))
}

func TestExternalEntityIsNotLoaded(t *testing.T) {
	t.Parallel()

	content := "<?xml version=\"1.0\"?>\n<!DOCTYPE tt [<!ENTITY xxe SYSTEM \"file:///etc/passwd\">]>\n" +
		`<tt xmlns="http://www.w3.org/ns/ttml"><body><div><p begin="0s" end="1s">a&xxe;b</p></div></body></tt>`
	assert.Equal(t, "The file is not well-formed XML.", readError(t, content, false).Msg)

	s, _ := mustRead(t, content, true)
	assert.Equal(t, []cueRow{{0, sec(1), "a&amp;xxe;b"}}, cueRows(s))
}

func TestExternalDTDIsNotLoaded(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "<?xml version=\"1.0\"?>\n<!DOCTYPE tt SYSTEM \"http://127.0.0.1:1/missing.dtd\">\n"+
		`<tt xmlns="http://www.w3.org/ns/ttml"><body><div><p begin="0s" end="1s">x</p></div></body></tt>`, false)
	assert.Equal(t, []cueRow{{0, sec(1), "x"}}, cueRows(s))
}

func TestDeclaredEncodingIsIgnored(t *testing.T) {
	t.Parallel()

	s, _ := mustRead(t, "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>\n"+
		"<tt xmlns=\"http://www.w3.org/ns/ttml\"><body><div><p begin=\"0s\" end=\"1s\">caf\u00e9</p></div></body></tt>", false)
	assert.Equal(t, []cueRow{{0, sec(1), "caf\u00e9"}}, cueRows(s))
}

func TestInvalidFile(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, content, want string }{
		{"not well-formed", `<tt xmlns="http://www.w3.org/ns/ttml"><body>`, "The file is not well-formed XML."},
		{"other root", "<html><body/></html>", "The root element is not a TTML <tt> element."},
		{"other namespace", `<tt xmlns="urn:example"/>`, "The root element is not a TTML <tt> element."},
		{"undeclared prefix on the root", `<x:tt/>`, "The root element is not a TTML <tt> element."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perr := readError(t, tt.content, false)
			assert.Equal(t, tt.want, perr.Msg)
			assert.Equal(t, model.NoLine, perr.Line)
		})
	}
}

func TestInvalidUTF8(t *testing.T) {
	t.Parallel()

	content := document(`<div><p end="1s">caf`+"\xe9"+`</p></div>`, "", "")
	perr := readError(t, content, false)
	assert.Contains(t, perr.Msg, "The content is not valid UTF-8.")

	s, warnings := mustRead(t, content, true)
	assert.Equal(t, []cueRow{{0, sec(1), "caf\ufffd"}}, cueRows(s))
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Message, "The parser read the bad bytes as U+FFFD.")
}

func TestEmptyContent(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, " \n ", false)
	assert.Equal(t, 0, s.Len())
	assert.Empty(t, warnings)

	s, _ = mustRead(t, document("", "", ""), false)
	assert.Equal(t, 0, s.Len())
}

func objectOf(t *testing.T, d *model.Data, key string) *model.Data {
	t.Helper()

	o, ok := d.Object(key)
	require.True(t, ok, key)
	return o
}

func stringOf(t *testing.T, d *model.Data, key string) string {
	t.Helper()

	v, ok := d.String(key)
	require.True(t, ok, key)
	return v
}

func intOf(t *testing.T, d *model.Data, key string) int64 {
	t.Helper()

	v, ok := d.Int(key)
	require.True(t, ok, key)
	return v
}

// Covers https://github.com/yama6a/subtitle-toolbox-go/issues/175: a character reference in an attribute value stays,
// and only literal white space becomes a space.
func TestAttributeReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, in, want, head string
	}{
		{"referenced LF", `a&#10;b`, "a\nb", `a&#10;b`},
		{"referenced TAB", `a&#9;b`, "a\tb", `a&#9;b`},
		{"referenced CR", `a&#13;b`, "a\rb", `a&#13;b`},
		{"literal LF", "c\nd", "c d", "c d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := `<tt xmlns="http://www.w3.org/ns/ttml"><head><styling><style xml:id="s" foo="` + tt.in + `"/></styling></head>` +
				`<body><div><p begin="1s" end="2s" foo="` + tt.in + `">x</p></div></body></tt>`
			s, _ := mustRead(t, in, false)
			c, _ := s.Cue(0)
			stored, _ := cueData(t, c).Object("attributes")
			assert.Equal(t, attrs("foo", tt.want), stored)
			head, _ := fileData(t, s).String("head")
			assert.Equal(t, `<head><styling><style xml:id="s" foo="`+tt.head+`"/></styling></head>`, head)

			var out bytes.Buffer
			require.NoError(t, ttml.Write(&out, s, model.CommonWriteOptions{}, ttml.WriteOptions{}))
			back, _ := mustRead(t, out.String(), false)
			backCue, _ := back.Cue(0)
			assert.Equal(t, cueData(t, c), cueData(t, backCue))
			backHead, _ := fileData(t, back).String("head")
			assert.Equal(t, head, backHead)
		})
	}
}
