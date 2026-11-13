package youtube_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports the end rules of tests/Parsers/YouTubeTimedTextParserTest.php of subtitle-toolbox-php.
func TestReadEnds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    [][2]int
	}{
		{
			name:    "srv1 seconds with fractions",
			content: `<transcript><text start="1.2" dur="2.3">Hello</text><text start="5" dur="1">World</text></transcript>`,
			want:    [][2]int{{1200, 3500}, {5000, 6000}},
		},
		{
			name:    "srv2 milliseconds",
			content: `<timedtext><text t="1200" d="2300">Hello</text></timedtext>`,
			want:    [][2]int{{1200, 3500}},
		},
		{
			name:    "a missing duration ends at the next later start, the last lasts 5 s",
			content: `<transcript><text start="1">a</text><text start="1">b</text><text start="4">c</text><text start="9">d</text></transcript>`,
			want:    [][2]int{{1000, 4000}, {1000, 4000}, {4000, 9000}, {9000, 14000}},
		},
		{
			name:    "a duration of 0 stays 0",
			content: `<transcript><text start="1" dur="0">a</text><text start="3" dur="1">b</text></transcript>`,
			want:    [][2]int{{1000, 1000}, {3000, 4000}},
		},
		{
			name:    "a time with white space and an exponent",
			content: `<transcript><text start=" 1e0 " dur=".5e1">a</text></transcript>`,
			want:    [][2]int{{1000, 6000}},
		},
		{
			name:    "srv3 keeps overlaps outside a window",
			content: `<timedtext format="3"><body><p t="0" d="3000">top</p><p t="1000" d="3000">bottom</p></body></timedtext>`,
			want:    [][2]int{{0, 3000}, {1000, 4000}},
		},
		{
			name: "srv3 ends in a window, an append is no later start",
			content: `<timedtext format="3"><body>` +
				`<p t="0" d="9000" w="1">a</p><p t="0" d="9000" w="2">b</p><p t="2000" d="9000" w="1">c</p>` +
				`<p t="3000" d="9000" w="1" a="1">` + "\n" + `</p><p t="4000" w="2">d</p><p t="4000" d="500" w="2">e</p>` +
				`</body></timedtext>`,
			want: [][2]int{{0, 2000}, {0, 4000}, {2000, 11000}, {4000, 9000}, {4000, 4500}},
		},
		{
			name:    "json3 without a duration lasts 0 s",
			content: `{"events": [{"tStartMs": 1000, "segs": [{"utf8": "a"}]}, {"tStartMs": 2000, "dDurationMs": 500, "segs": [{"utf8": "b"}]}]}`,
			want:    [][2]int{{1000, 1000}, {2000, 2500}},
		},
		{
			name:    "any element name counts",
			content: `<transcript><line start="1" dur="1">a</line></transcript>`,
			want:    [][2]int{{1000, 2000}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, times(read(t, tt.content, youtube.ReadOptions{})))
		})
	}
}

func TestReadTextDecodesEntitiesTwice(t *testing.T) {
	t.Parallel()

	s := read(t, `<transcript><text start="0" dur="1">it&amp;#39;s &amp;amp; more</text></transcript>`, youtube.ReadOptions{})
	c, _ := s.Cue(0)
	assert.Equal(t, []string{"it's &amp; more"}, c.Lines)
	data, ok := c.FormatData("youtube")
	require.True(t, ok)
	text, _ := data.String("text")
	assert.Equal(t, "it&#39;s &amp; more", text)
}

func TestReadXMLErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"not well-formed", `<transcript><text start="1"></transcript>`, "The content is not well-formed XML."},
		{"another root", `<tt/>`, "The root element must be <timedtext> or <transcript>, not <tt>."},
		{"no start", `<transcript><text dur="1">a</text></transcript>`, `The <text> element has no valid "start" attribute.`},
		{"text as start", `<transcript><text start="x">a</text></transcript>`, `The <text> element has no valid "start" attribute.`},
		{"negative start", `<transcript><text start="-1">a</text></transcript>`, `The <text> element has no valid "start" attribute.`},
		{"bad duration", `<timedtext><text t="1" d="1x">a</text></timedtext>`, `The <text> element has no valid "d" attribute.`},
		{"start over the limit", `<transcript><text start="360000000">a</text></transcript>`, `The time "360000000" is not below 100000 hours.`},
		{"end over the limit", `<transcript><text start="359999999" dur="2">a</text></transcript>`, `The time "2" is not below 100000 hours.`},
		{"text before the XML", "credit\n<transcript/>", `The file has text before the XML: "credit".`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := youtube.Read(strings.NewReader(tt.content), model.CommonReadOptions{}, youtube.ReadOptions{})
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.want, perr.Msg)
		})
	}
}

func TestReadLenientSkipsABadCaption(t *testing.T) {
	t.Parallel()

	content := "<transcript>\n<text start=\"1\" dur=\"1\">a</text>\n<text start=\"x\">b</text>\n<text start=\"3\" dur=\"1\">c</text></transcript>"
	s, warnings, err := youtube.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, youtube.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, s.Len())
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Skipped, warnings[0].Action)
	assert.Equal(t, 3, warnings[0].Line)
	assert.Equal(t, 1, warnings[0].Block)
	assert.Equal(t, []string{`<text start="x">b</text>`}, warnings[0].Lines)
}

func TestReadAppliesTheLastCueDuration(t *testing.T) {
	t.Parallel()

	d := ms(2500)
	s, _, err := youtube.Read(strings.NewReader(`<transcript><text start="1">a</text></transcript>`),
		model.CommonReadOptions{LastCueDuration: &d}, youtube.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, [][2]int{{1000, 3500}}, times(s))
}
