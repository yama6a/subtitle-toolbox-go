package sprucestl_test

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/sprucestl"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const defaultHeader = "$FontName = Arial\n$FontSize = 42\n$HorzAlign = Center\n$VertAlign = Bottom\n$XOffset = 0\n$YOffset = 0\n" +
	"$Bold = False\n$Italic = False\n$Underlined = False\n$TextContrast = 15\n$Outline1Contrast = 15\n" +
	"$Outline2Contrast = 15\n$BackgroundContrast = 0\n$ForceDisplay = False\n$FadeIn = 0\n$FadeOut = 0\n" +
	"$TapeOffset = False\n\n"

// Ports the formatter tests of tests/Formatters/SpruceStlFormatterTest.php of subtitle-toolbox-php, and the escapes
// of docs/formats.md.
func TestWrite(t *testing.T) {
	t.Parallel()

	signAlignment := model.Cue{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"Sign"}, Position: model.Position{Alignment: model.TopLeft}}
	tests := []struct {
		name string
		s    *model.Subtitle
		opts sprucestl.WriteOptions
		want string
	}{
		{
			name: "a new file gets the default header and 25 fps",
			s:    subtitle(cue(137400*ms, 140360*ms, "The bakery opens at seven,", "every day of the week.")),
			want: defaultHeader + "00:02:17:10 , 00:02:20:09 , The bakery opens at seven,|every day of the week.\n",
		},
		{
			name: "toggles for bold, italics and underline",
			s:    subtitle(cue(time.Second, 2*time.Second, "<i>Over the water,</i>", `<i>the <b>gulls</b></i> <u>fly</u> &amp; <font color="red">sing</font>`)),
			want: defaultHeader + "00:00:01:00 , 00:00:02:00 , ^IOver the water,|the ^Bgulls ^B^I^Ufly^U & sing\n",
		},
		{
			name: "toggles against the style of the header",
			s:    withLines(t, mustRead(t, "$Italic = True\n00:00:01:00 , 00:00:02:00 , Sung\n"), "Spoken <i>sung</i>"),
			want: "$Italic = True\n00:00:01:00 , 00:00:02:00 , ^ISpoken ^Isung\n",
		},
		{
			name: "alignment commands when the alignment changes",
			s:    withCues(mustRead(t, "00:00:01:00 , 00:00:02:00 , One\n"), signAlignment, cue(5*time.Second, 6*time.Second, "Two")),
			want: "00:00:01:00 , 00:00:02:00 , One\n$HorzAlign = Left\n$VertAlign = Top\n00:00:03:00 , 00:00:04:00 , Sign\n$HorzAlign = Center\n$VertAlign = Bottom\n00:00:05:00 , 00:00:06:00 , Two\n",
		},
		{
			name: "comments after the last cue",
			s:    withComment(mustRead(t, "00:00:01:00,00:00:02:00,One\n"), "Reel 2\nchecked", 1),
			want: "00:00:01:00,00:00:02:00,One\n//Reel 2\n//checked\n",
		},
		{
			name: "the option rate wins",
			s:    mustRead(t, "00:00:01:15 , 00:00:02:00 , PAL\n"),
			opts: sprucestl.WriteOptions{FrameRate: rate30},
			want: "00:00:01:18 , 00:00:02:00 , PAL\n",
		},
		{
			name: "a frame at the frame rate rolls up",
			s:    subtitle(cue(time.Second, 2990*ms, "One")),
			opts: sprucestl.WriteOptions{FrameRate: rate25},
			want: defaultHeader + "00:00:01:00 , 00:00:03:00 , One\n",
		},
		{
			name: "the last time below 100000 hours stops at the last frame",
			s:    subtitle(cue(timeNearLimit, timeNearLimit, "One")),
			opts: sprucestl.WriteOptions{FrameRate: rate25},
			want: defaultHeader + "99999:59:59:24 , 99999:59:59:24 , One\n",
		},
		{
			name: "a | and a toggle look-alike in the text",
			s:    subtitle(cue(time.Second, 2*time.Second, "Rock | Pop", "^Bold ^Italic ^Underline", "a^<i>B</i>", `x^<font color="red">Bold</font>`)),
			want: defaultHeader + "00:00:01:00 , 00:00:02:00 , Rock \u00a6 Pop|^\u2060Bold ^\u2060Italic ^\u2060Underline|a^^IB|^Ix^\u2060Bold\n",
		},
		{
			name: "a line break inside a line",
			s:    subtitle(cue(time.Second, 2*time.Second, "a\nb", "c&#10;d")),
			want: defaultHeader + "00:00:01:00 , 00:00:02:00 , a|b|c|d\n",
		},
		{
			name: "a stored header, command and trailer line with CR or LF",
			s: withFileData(subtitle(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"One"}}),
				model.NewData(model.Field{Key: "header", Value: []any{"$A = 1\r\n$B = 2"}}, model.Field{Key: "trailer", Value: []any{"$C = 3\n"}})),
			want: "$A = 1  $B = 2\n00:00:01:00 , 00:00:02:00 , One\n$C = 3 \n",
		},
		{
			name: "stored lines that the reader would not read as that kind are skipped",
			s: withFileData(subtitle(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"One"}}),
				model.NewData(model.Field{Key: "header", Value: []any{"Hello", "//c", "", "00:00:01:00 , 00:00:02:00 , Two"}}, model.Field{Key: "trailer", Value: []any{"//x", "$T = 1"}})),
			want: "//c\n\n00:00:01:00 , 00:00:02:00 , One\n$T = 1\n",
		},
		{
			name: "frame 24 stays at 24.4 fps",
			s:    subtitle(cue(time.Second, 1984*ms, "One"), cue(2*time.Second, 2999*ms, "Two")),
			opts: sprucestl.WriteOptions{FrameRate: model.FrameRate{Num: 122, Den: 5}},
			want: defaultHeader + "00:00:01:00 , 00:00:01:24 , One\n00:00:02:00 , 00:00:02:24 , Two\n",
		},
		{
			name: "an empty stored header writes no header",
			s:    withFileData(subtitle(cue(time.Second, 2*time.Second, "One")), model.NewData(model.Field{Key: "header", Value: []any{}})),
			want: "00:00:01:00 , 00:00:02:00 , One\n",
		},
		{
			name: "a stored separator and line break that do not match the shape fall back",
			s: withFileData(subtitle(cue(time.Second, 2*time.Second, "One", "Two")),
				model.NewData(model.Field{Key: "header", Value: []any{}}, model.Field{Key: "separator", Value: ";"}, model.Field{Key: "lineBreak", Value: "||"})),
			want: "00:00:01:00 , 00:00:02:00 , One|Two\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, tt.s, model.CommonWriteOptions{}, tt.opts))
		})
	}
}

const timeNearLimit = 100000*time.Hour - time.Millisecond

func TestWriteRoundTrips(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{"a $ line after the last cue", "$VertAlign = Bottom\n00:00:01:00 , 00:00:02:00 , One\n$VertAlign = Top\n"},
		{"a $ line after the last comment", "00:00:01:00 , 00:00:02:00 , One\n//end\n$VertAlign = Top\n"},
		{"a header without cues", "$A = 1\n\n"},
		{"a tab separator and a spaced line break", "00:00:01:00\t,\t00:00:02:00\t,\tOne | two\n"},
		{"the look-alikes of structure", "00:00:01:00 , 00:00:02:00 , Rock \u00a6 Pop|^\u2060Bold\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := mustRead(t, tt.content)
			assert.Equal(t, tt.content, write(t, s, model.CommonWriteOptions{}, sprucestl.WriteOptions{}))
		})
	}
}

func TestWriteKeepsTheLineEndOfACRLFFile(t *testing.T) {
	t.Parallel()

	content := "$Italic = False\r\n\r\n//note\r\n00:00:01:00 , 00:00:02:00 , One\r\n$Italic = True\r\n00:00:03:00 , 00:00:04:00 , Two\r\n"
	got := write(t, mustRead(t, content), model.CommonWriteOptions{LineEnding: model.CRLF}, sprucestl.WriteOptions{})
	assert.Equal(t, content, got)
	assert.False(t, strings.HasPrefix(got, "\ufeff"))
}

func TestWriteChecksTheFrameRate(t *testing.T) {
	t.Parallel()

	s := withFileData(subtitle(cue(time.Second, 2*time.Second, "One")), model.NewData(model.Field{Key: "frameRate", Value: -1.0}))
	err := writeErr(s, sprucestl.WriteOptions{})
	var aerr *model.InvalidArgumentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, "WriteOptions.SpruceSTL.FrameRate", aerr.Option)
	assert.Equal(t, "The Spruce STL frame rate must be greater than 0, got -1.", aerr.Msg)
	require.ErrorAs(t, writeErr(subtitle(), sprucestl.WriteOptions{FrameRate: model.FrameRate{Num: 0, Den: 1}}), &aerr)
}

func withLines(t *testing.T, s *model.Subtitle, lines ...string) *model.Subtitle {
	t.Helper()

	c, _ := s.Cue(0)
	c.Lines = lines
	require.NoError(t, s.SetCue(0, c))
	return s
}

func withCues(s *model.Subtitle, cues ...model.Cue) *model.Subtitle {
	s.AddCues(cues...)
	return s
}

func withComment(s *model.Subtitle, text string, beforeCue int) *model.Subtitle {
	_ = s.AddComment(text, beforeCue)
	return s
}

func withFileData(s *model.Subtitle, data *model.Data) *model.Subtitle {
	_ = s.SetFormatData("spruce-stl", data)
	return s
}

func writeErr(s *model.Subtitle, opts sprucestl.WriteOptions) error {
	_, err := sprucestl.Write(io.Discard, s, model.CommonWriteOptions{}, opts)
	return err
}

func TestWriteRejectsAFrameRateAbove100(t *testing.T) {
	t.Parallel()

	err := writeErr(subtitle(cue(time.Second, 2*time.Second, "One")), sprucestl.WriteOptions{FrameRate: model.FrameRate{Num: 101, Den: 1}})
	var uerr *model.UnwritableError
	require.ErrorAs(t, err, &uerr)
	assert.Contains(t, uerr.Reason, "WriteOptions.SpruceSTL.FrameRate")
	require.NoError(t, writeErr(subtitle(cue(time.Second, 2*time.Second, "One")), sprucestl.WriteOptions{FrameRate: model.FrameRate{Num: 100, Den: 1}}))
}
