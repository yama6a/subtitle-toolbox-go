package markupcore_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

func TestDecodeEntities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want string
	}{
		{"&lt;b&gt;1 &amp; 2&lt;/b&gt; &quot;it&#39;s&quot;", `<b>1 & 2</b> "it's"`},
		{"Caf&eacute; &nbsp;", "Caf\u00e9 \u00a0"},
		{"plain & simple", "plain & simple"},
		// The rows below are measured on PHP 8.5 with html_entity_decode(ENT_QUOTES | ENT_HTML5).
		{"&#9;", "\t"},
		{"&#12;", "\f"},
		{"&#160;", "\u00a0"},
		{"&#233;", "\u00e9"},
		{"&#xE9;", "\u00e9"},
		{"&#x10FFFD;", "\U0010fffd"},
		{"&#0;", "&#0;"},
		{"&#11;", "&#11;"},
		{"&#13;", "&#13;"},
		{"&#31;", "&#31;"},
		{"&#127;", "&#127;"},
		{"&#128;", "&#128;"},
		{"&#x9F;", "&#x9F;"},
		{"&#xD800;", "&#xD800;"},
		{"&#xFDD0;", "&#xFDD0;"},
		{"&#xFFFE;", "&#xFFFE;"},
		{"&#x1FFFF;", "&#x1FFFF;"},
		{"&#x10FFFF;", "&#x10FFFF;"},
		{"&#x110000;", "&#x110000;"},
		{"&amp", "&amp"},
		{"&Amp;", "&Amp;"},
		{"&notit;", "&notit;"},
		{"&AMP;", "&"},
		{"&apos;", "'"},
		{"&#X41;&#00065;", "AA"},
		{"&#xFDCF;&#xFDF0;", "\ufdcf\ufdf0"},
		{"&#9999999999999999999;", "&#9999999999999999999;"},
		{"&#x;&#;&;", "&#x;&#;&;"},
		{"&amp&amp;", "&amp&"},
		{"&amp;amp;", "&amp;"},
		{"&nGt;&nGtv;", "\u226b\u20d2\u226b\u0338"},
		{"&#x0x41; &#X0X42; &#x0X9;", "A B \t"},
		{"&#x00x41; &#x0x; &#x0xg; &#x0x0x41; &#0x41;", "&#x00x41; &#x0x; &#x0xg; &#x0x0x41; &#0x41;"},
		{"caf\xe9 &amp;", "caf\xe9 &"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.DecodeEntities(tt.text))
		})
	}
}

func TestEscapeText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want string
	}{
		{"<b>1 & 2</b>", "&lt;b&gt;1 &amp; 2&lt;/b&gt;"},
		{"&amp; stays text", "&amp;amp; stays text"},
		{"\"it's\" Caf\u00e9", "\"it's\" Caf\u00e9"},
		{"", ""},
		{"caf\xe9 & <b>", "caf\xe9 &amp; &lt;b&gt;"},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.EscapeText(tt.text))
		})
	}
}

func TestEscapeThenDecodeRoundTrips(t *testing.T) {
	t.Parallel()

	text := "<b>1 & 2</b> &amp;"
	assert.Equal(t, text, markupcore.DecodeEntities(markupcore.StripAllTags(markupcore.EscapeText(text))))
	assert.Equal(t, "<b> is bold", markupcore.DecodeEntities(markupcore.StripAllTags("<i>&lt;b&gt;</i> is bold")))
}

func TestUnescapeText(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "<b> & > &eacute; &amp;", markupcore.UnescapeText("&lt;b&gt; &amp; &gt; &eacute; &amp;amp;"))
	assert.Equal(t, "1 < 2 & 3", markupcore.UnescapeText(markupcore.EscapeText("1 < 2 & 3")))
}

func TestEscapeTextLike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		raw  string
		want string
	}{
		{"escaped raw", "Tom & Jerry > 2", "Tom &amp; Jerry &gt; 1", "Tom &amp; Jerry &gt; 2"},
		{"raw ampersand and greater-than sign", "Tom & Jerry > 2", "Tom & Jerry > 1", "Tom & Jerry > 2"},
		{"entity not in raw", "&eacute; & <", "a & b", "&amp;eacute; & &lt;"},
		{"entity in raw", "&nbsp;x", "&nbsp;y", "&nbsp;x"},
		{"entity in another case", "&NBSP;x", "&nbsp;y", "&nbsp;x"},
		{"exact form among several", "&nbsp;x", "&NBSP; &nbsp;y", "&nbsp;x"},
		{"several other forms", "&nbsp;x", "&NBSP; &Nbsp;y", "&amp;nbsp;x"},
		{"single other form", "&nbsp;x", "&NBSP;y", "&NBSP;x"},
		{"entity escaped as text in raw", "&nbsp;x", "&amp;nbsp; &nbsp;", "&amp;nbsp;x"},
		{"escape entities count as text", "&lt; &amp; &gt;", "&lt; &amp; &gt;", "&amp;lt; &amp;amp; &amp;gt;"},
		{"ampersand only in an entity of raw", "a&b", "&nbsp;", "a&amp;b"},
		{"bare ampersand next to an entity of raw", "&eacute;", "&eacute; &", "&eacute;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.EscapeTextLike(tt.text, tt.raw))
		})
	}
}

func TestNamedEntity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		want   string
		wantOK bool
	}{
		{"eacute", "é", true},
		{"NonBreakingSpace", " ", true},
		{"amp", "&", true},
		{"nGt", "≫⃒", true},
		{"eacute;", "", false},
		{"cur", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := markupcore.NamedEntity(tt.name)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
