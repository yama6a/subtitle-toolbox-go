package youtube_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func replaceLines(c *model.Cue, old, with string) {
	for i, l := range c.Lines {
		c.Lines[i] = strings.ReplaceAll(l, old, with)
	}
}

// Ports the edit tests of tests/Formatters/YouTubeTimedTextFormatterTest.php of subtitle-toolbox-php for srv1 and srv3.
func TestWriteAfterEdits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, file string
		opts       youtube.ReadOptions
		edit       func(t *testing.T, s *model.Subtitle)
		want       []string
	}{
		{
			name: "an edited text of srv1 loses its escaped entities and the others keep them", file: "transcript.en.srv1", opts: withWords,
			edit: func(t *testing.T, s *model.Subtitle) {
				t.Helper()
				editCue(t, s, 0, func(c *model.Cue) { c.SetText("hello all and welcome back"); c.Words = nil })
			},
			want: []string{
				`<?xml version="1.0" encoding="utf-8" ?><transcript><text start="0.16" dur="4.4">hello all and welcome back</text>` +
					`<text start="2.96" dur="4.48">today we&amp;#39;re planting tomatoes</text>`,
			},
		},
		{
			name: "a new style gets a new pen and an equal style shares it", file: "styled.en.srv3", opts: withWords,
			edit: func(t *testing.T, s *model.Subtitle) {
				t.Helper()
				editCue(t, s, 1, func(c *model.Cue) { c.SetText("<i>The ferry</i> leaves at <u>noon</u>.") })
				editCue(t, s, 2, func(c *model.Cue) { replaceLines(c, "four euros", "five euros") })
			},
			want: []string{
				"<pen id=\"4\" i=\"1\"/>\n<pen id=\"5\" u=\"1\"/>\n",
				`<p t="2500" d="3000" wp="2" ws="1"><s p="4">The ferry</s> leaves at <s p="5">noon</s>.</p>`,
				`<p t="5500" d="3200" wp="2" ws="1"><s p="1">Tickets cost </s><s p="2">five euros</s><s p="1">.</s></p>`,
				`<s p="3">Pier 3 &amp;#39;North&amp;#39;</s>`,
			},
		},
		{
			name: "a new alignment gets a window position", file: "styled.en.srv3", opts: withWords,
			edit: func(t *testing.T, s *model.Subtitle) {
				t.Helper()
				editCue(t, s, 4, func(c *model.Cue) { c.Position.Alignment = model.TopRight })
				editCue(t, s, 1, func(c *model.Cue) { c.Position.Alignment = model.TopLeft })
			},
			want: []string{
				"<wp id=\"3\" ap=\"0\" ah=\"0\" av=\"0\"/>\n<wp id=\"4\" ap=\"2\" ah=\"100\" av=\"0\"/>\n",
				`<p t="2500" d="3000" p="1" wp="3" ws="1">`,
				"<p t=\"11500\" d=\"3000\" wp=\"4\">Please keep\nyour ticket.</p>",
			},
		},
		{
			name: "an edited word keeps the other words and the append events", file: "auto.en.srv3", opts: withWords,
			edit: func(t *testing.T, s *model.Subtitle) {
				t.Helper()
				editCue(t, s, 1, func(c *model.Cue) {
					replaceLines(c, "plant", "sow")
					c.Words[2].Text = "sow"
				})
				require.NoError(t, s.Shift(1000*ms(1)))
			},
			want: []string{
				"<p t=\"3950\" d=\"1610\" w=\"1\" a=\"1\">\n</p>\n" +
					`<p t="3960" d="4480" w="1"><s ac="0">today</s><s t="320" ac="0"> we</s><s t="480"> sow</s><s t="800" ac="0"> tomatoes</s></p>`,
			},
		},
		{
			name: "a word changed without word times keeps the offset of its segment", file: "auto.en.srv3", opts: youtube.ReadOptions{},
			edit: func(t *testing.T, s *model.Subtitle) {
				t.Helper()
				editCue(t, s, 1, func(c *model.Cue) { c.SetText("today we sow tomatoes") })
			},
			want: []string{
				`<p t="2960" d="4480" w="1"><s ac="0">today</s><s t="320" ac="0"> we</s><s t="480"> sow</s><s t="800" ac="0"> tomatoes</s></p>`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := readFile(t, tt.file, tt.opts)
			tt.edit(t, s)
			out := write(t, s)
			for _, want := range tt.want {
				assert.Contains(t, out, want)
			}
		})
	}
}

func TestWriteMovedCueKeepsItsDuration(t *testing.T) {
	t.Parallel()

	s, _ := readFile(t, "missing-d.en.srv2", withWords)
	editCue(t, s, 0, func(c *model.Cue) { c.Start, c.End = ms(1500), ms(3500) })
	editCue(t, s, 1, func(c *model.Cue) { c.End = ms(6000) })
	want := "<?xml version=\"1.0\" encoding=\"utf-8\" ?>\n<timedtext>\n<text t=\"1500\" d=\"2000\">the market opens at eight</text>\n" +
		"<text t=\"4000\" d=\"2000\">bring a basket</text>\n<text t=\"6500\" d=\"0\">and some coins</text>\n" +
		"<text t=\"8000\">we close at noon</text>\n</timedtext>\n"
	assert.Equal(t, want, write(t, s))
}

func fromAnotherFormat(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func TestWriteCuesFromAnotherFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    func() *model.Subtitle
		want string
	}{
		{
			name: "srv1 when the format data says so",
			s: func() *model.Subtitle {
				s := fromAnotherFormat(
					model.Cue{Start: ms(1500), End: ms(4000), Lines: []string{"<i>Fish &amp; chips</i>", "today"}},
					model.Cue{Start: ms(5000), End: ms(7250), Lines: []string{"Done"}})
				require.NoError(t, s.SetFormatData("youtube", model.NewData(model.Field{Key: "format", Value: "srv1"})))
				return s
			},
			want: `<?xml version="1.0" encoding="utf-8" ?><transcript><text start="1.5" dur="2.5">Fish &amp; chips` + "\n" +
				`today</text><text start="5" dur="2.25">Done</text></transcript>`,
		},
		{
			name: "srv3 with pens, a window position and word times",
			s: func() *model.Subtitle {
				first := model.Cue{Start: ms(1000), End: ms(2000), Lines: []string{"<i>Hi</i> there"}}
				first.Words = []model.Word{{Text: "Hi", Start: ms(1000), End: ms(1500)}, {Text: "there", Start: ms(1500), End: ms(2000)}}
				second := model.Cue{Start: ms(3000), End: ms(4000), Lines: []string{`<font color="red">Red</font>`, "two"}}
				second.Position.Alignment = model.TopLeft
				return fromAnotherFormat(first, second)
			},
			want: "<?xml version=\"1.0\" encoding=\"utf-8\" ?><timedtext format=\"3\">\n<head>\n<pen id=\"1\" i=\"1\"/>\n<pen id=\"2\" fc=\"#FF0000\"/>\n" +
				"<wp id=\"1\" ap=\"0\" ah=\"0\" av=\"0\"/>\n</head>\n<body>\n<p t=\"1000\" d=\"1000\"><s p=\"1\">Hi</s><s t=\"500\"> there</s></p>\n" +
				"<p t=\"3000\" d=\"1000\" wp=\"1\"><s p=\"2\">Red</s>\ntwo</p>\n</body>\n</timedtext>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, tt.s()))
		})
	}
}

func TestWriteSrv3EscapesTheTextOfAChangedCueOnce(t *testing.T) {
	t.Parallel()

	s := fromAnotherFormat(model.Cue{
		Start: ms(0), End: ms(2000), Lines: []string{"Fish &amp; chips"},
		Words: []model.Word{{Text: "chips", Start: ms(500), End: ms(2000)}},
	})
	out := write(t, s)
	assert.Contains(t, out, "<p t=\"0\" d=\"2000\">Fish &amp;<s t=\"500\"> chips</s></p>")
	assert.NotContains(t, out, "&amp;amp;")
}
