package markupcore_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

func TestStyleRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		runs          []markupcore.Run
		spacesOutside bool
		want          string
	}{
		{
			"nests font, b, i, u, s",
			[]markupcore.Run{{Text: "a", Color: "#ff0000", Bold: true, Italic: true, Underline: true, Strike: true}},
			false,
			`<font color="#ff0000"><b><i><u><s>a</s></u></i></b></font>`,
		},
		{
			"closes from the first style that changes",
			[]markupcore.Run{
				{Text: "a", Color: "#ff0000", Italic: true},
				{Text: "b", Color: "#ff0000", Underline: true},
				{Text: "c", Italic: true},
				{Text: "d"},
			},
			false,
			`<font color="#ff0000"><i>a</i><u>b</u></font><i>c</i>d`,
		},
		{
			"outer style stays open",
			[]markupcore.Run{{Text: "Hi ", Bold: true}, {Text: "there", Bold: true, Italic: true}},
			false,
			"<b>Hi <i>there</i></b>",
		},
		{
			"bold opens outside italic",
			[]markupcore.Run{{Text: "Hi ", Italic: true}, {Text: "there", Italic: true, Bold: true}},
			false,
			"<i>Hi </i><b><i>there</i></b>",
		},
		{
			"escapes text and keeps the tags over white space",
			[]markupcore.Run{{Text: "a & ", Italic: true}, {Text: ""}, {Text: "  ", Bold: true}, {Text: "b", Italic: true}, {Text: " "}},
			false,
			"<i>a &amp;   b </i>",
		},
		{
			"spaces outside",
			[]markupcore.Run{{Text: " a ", Italic: true}, {Text: " "}, {Text: "b", Bold: true}, {Text: " c \t", Bold: true}},
			true,
			" <i>a</i>  <b>b c</b> \t",
		},
		{"spaces outside and only white space", []markupcore.Run{{Text: "  ", Italic: true}}, true, "  "},
		{"vertical tab is white space", []markupcore.Run{{Text: "a\v", Italic: true}}, true, "<i>a</i>\v"},
		{"form feed is white space", []markupcore.Run{{Text: "\fa", Italic: true}}, true, "\f<i>a</i>"},
		{"no runs", nil, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.StyleRuns(tt.runs, tt.spacesOutside))
		})
	}
}
