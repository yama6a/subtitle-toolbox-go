package sprucestl_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/sprucestl"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// Ports the toggle, frame rate and tab tests of tests/Parsers/SpruceStlParserTest.php of subtitle-toolbox-php.
// The tab and hours row adapts testClientFile and testTimesBiggerThan24Hours of mantas-done/subtitles,
// tests/formats/StlTest.php
// (https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/formats/StlTest.php), MIT.
func TestRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		fps      model.FrameRate
		lenient  bool
		want     []cueRow
		wantRate float64
		warnings []string
	}{
		{
			name:     "toggles flip the style of the commands for each cue",
			content:  "$Italic = TRUE\n00:00:01:00 , 00:00:02:00 , Sung ^Ispoken^I sung|still sung\n00:00:03:00 , 00:00:04:00 , ^IPlain\n",
			want:     []cueRow{{time.Second, 2 * time.Second, []string{"<i>Sung</i> spoken <i>sung</i>", "<i>still sung</i>"}, model.BottomCenter}, {3 * time.Second, 4 * time.Second, []string{"Plain"}, model.BottomCenter}},
			wantRate: 25,
		},
		{
			name:     "a toggle in cue text escapes < and &",
			content:  "00:00:01:00 , 00:00:02:00 , a < b ^B& c^B\n",
			want:     []cueRow{{time.Second, 2 * time.Second, []string{"a &lt; b <b>&amp; c</b>"}, model.BottomCenter}},
			wantRate: 25,
		},
		{
			name:     "alignment commands move the cues",
			content:  "$HorzAlign = Right\n$VertAlign = center\n00:00:01:00 , 00:00:02:00 , One\n$VertAlign = Top\n00:00:03:00 , 00:00:04:00 , Two\n",
			want:     []cueRow{{time.Second, 2 * time.Second, []string{"One"}, model.MiddleRight}, {3 * time.Second, 4 * time.Second, []string{"Two"}, model.TopRight}},
			wantRate: 25,
		},
		{
			name:     "a frame number of 25 or more means 30 fps",
			content:  "00:00:01:15 , 00:00:02:25 , NTSC\n",
			want:     []cueRow{{1500 * ms, 2833 * ms, []string{"NTSC"}, model.BottomCenter}},
			wantRate: 30,
		},
		{
			name:     "frame numbers below 25 mean 25 fps, without a warning in strict mode",
			content:  "00:00:01:15 , 00:00:02:24 , PAL\n",
			want:     []cueRow{{1600 * ms, 2960 * ms, []string{"PAL"}, model.BottomCenter}},
			wantRate: 25,
		},
		{
			name:     "lenient mode warns about the guessed frame rate",
			content:  "00:00:01:15 , 00:00:02:00 , PAL\n",
			lenient:  true,
			want:     []cueRow{{1600 * ms, 2 * time.Second, []string{"PAL"}, model.BottomCenter}},
			wantRate: 25,
			warnings: []string{"The file does not give its frame rate. The parser read the frames at 25 fps."},
		},
		{
			name:     "the option wins and gives no warning",
			content:  "00:00:01:12 , 00:00:02:00 , Film\n",
			fps:      model.FrameRate{Num: 24, Den: 1},
			lenient:  true,
			want:     []cueRow{{1500 * ms, 2 * time.Second, []string{"Film"}, model.BottomCenter}},
			wantRate: 24,
		},
		{
			name:     "a file without cues gives no warning",
			content:  "$Bold = True\n",
			lenient:  true,
			want:     []cueRow{},
			wantRate: 25,
		},
		{
			name:     "an end before the start swaps in lenient mode",
			content:  "00:00:02:00 , 00:00:01:00 , Back\n",
			fps:      rate25,
			lenient:  true,
			want:     []cueRow{{time.Second, 2 * time.Second, []string{"Back"}, model.BottomCenter}},
			wantRate: 25,
			warnings: []string{"The cue ends at 1 s, before it starts at 2 s. The parser swapped the times."},
		},
		{
			name:     "mantas-done tabs around the commas and hours above 24",
			content:  "$FadeOut\t\t\t=\t0\n$TapeOffset\t\t\t=\tTRUE\n \n$HorzAlign\t=\tCenter\n00:00:00:00\t,\t00:00:01:00\t,\ta\n00:00:01:00\t,\t00:00:02:00\t,\tb\n99:00:00:00 , 99:00:01:00 , c\n",
			want:     []cueRow{{0, time.Second, []string{"a"}, model.BottomCenter}, {time.Second, 2 * time.Second, []string{"b"}, model.BottomCenter}, {356400 * time.Second, 356401 * time.Second, []string{"c"}, model.BottomCenter}},
			wantRate: 25,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := read(tt.content, tt.lenient, tt.fps)
			require.NoError(t, err)
			assert.Equal(t, tt.want, cueRows(s))
			data, ok := s.FormatData("spruce-stl")
			require.True(t, ok)
			got, _ := data.Number("frameRate")
			assert.InDelta(t, tt.wantRate, got, 0)
			var messages []string
			for _, w := range warnings {
				messages = append(messages, w.Message)
			}
			assert.Equal(t, tt.warnings, messages)
		})
	}
}

func TestReadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		fps         model.FrameRate
		wantMsg     string
		wantLine    int
		wantWarning string
		wantCues    int
	}{
		{
			name:        "a line that is no subtitle",
			content:     "00:00:01:00 , 00:00:02:00 , One\n00:00:03:00 . 00:00:04:00 , Two\n00:00:05:00 , 00:00:06:00 , Three\n",
			wantMsg:     `The line "00:00:03:00 . 00:00:04:00 , Two" is no Spruce STL subtitle, command or comment.`,
			wantLine:    2,
			wantWarning: `The line "00:00:03:00 . 00:00:04:00 , Two" is no Spruce STL subtitle, command or comment.`,
			wantCues:    2,
		},
		{
			name:        "a frame number at the frame rate",
			content:     "00:00:01:25 , 00:00:02:00 , Late\n",
			fps:         rate25,
			wantMsg:     `The frame number in "00:00:01:25" is not below the frame rate of 25 fps.`,
			wantLine:    1,
			wantWarning: `The frame number in "00:00:01:25" is not below the frame rate of 25 fps.`,
		},
		{
			name:        "a time of 100000 hours",
			content:     "100000:00:00:00 , 100000:00:01:00 , Far\n",
			fps:         rate25,
			wantMsg:     `The time "100000:00:00:00" is not below 100000 hours.`,
			wantLine:    1,
			wantWarning: `The time "100000:00:00:00" is not below 100000 hours.`,
		},
		{
			name:        "an end before the start",
			content:     "00:00:02:00 , 00:00:01:00 , Back\n",
			fps:         rate25,
			wantMsg:     "The cue ends at 1 s, before it starts at 2 s.",
			wantLine:    1,
			wantWarning: "The cue ends at 1 s, before it starts at 2 s. The parser swapped the times.",
			wantCues:    1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := read(tt.content, false, tt.fps)
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.wantMsg, perr.Msg)
			assert.Equal(t, tt.wantLine, perr.Line)

			s, warnings, err := read(tt.content, true, tt.fps)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCues, s.Len())
			var got *model.ParseWarning
			for _, w := range warnings {
				if w.Message == tt.wantWarning {
					got = &w
				}
			}
			require.NotNil(t, got, "warnings %v", warnings)
			assert.Equal(t, tt.wantLine, got.Line)
			assert.Equal(t, model.NoBlock, got.Block)
		})
	}
}

func TestSkippedLineWarning(t *testing.T) {
	t.Parallel()

	_, warnings, err := read("00:00:01:00 , 00:00:02:00 , One\nstray\n", true, rate25)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Skipped, warnings[0].Action)
	assert.Equal(t, []string{"stray"}, warnings[0].Lines)
}

func TestReadKeepsTheFileShapeInTheFormatData(t *testing.T) {
	t.Parallel()

	s := mustRead(t, "$A = 1\n\n00:00:01:00\t,\t00:00:02:00\t,\tOne | two\n$B = 2\n//note\n$C = 3\n00:00:03:00\t,\t00:00:04:00\t,\tThree\n$D = 4\n\n")
	data, _ := s.FormatData("spruce-stl")
	header, _ := data.Strings("header")
	assert.Equal(t, []string{"$A = 1", ""}, header)
	separator, _ := data.String("separator")
	assert.Equal(t, "\t,\t", separator)
	lineBreak, _ := data.String("lineBreak")
	assert.Equal(t, " | ", lineBreak)
	trailer, _ := data.Strings("trailer")
	assert.Equal(t, []string{"$D = 4"}, trailer)
	second, _ := s.Cue(1)
	cueData, _ := second.FormatData("spruce-stl")
	commands, _ := cueData.Strings("commands")
	assert.Equal(t, []string{"$B = 2", "$C = 3"}, commands)
	require.Len(t, s.Comments(), 1)
	assert.Equal(t, model.Comment{Text: "note", BeforeCue: 1}, s.Comments()[0])
}

func TestOptionsValidate(t *testing.T) {
	t.Parallel()

	bad := model.FrameRate{Num: -1, Den: 1}
	var rerr, werr *model.InvalidArgumentError
	require.ErrorAs(t, sprucestl.ReadOptions{FrameRate: bad}.Validate(), &rerr)
	assert.Equal(t, "ReadOptions.SpruceSTL.FrameRate", rerr.Option)
	assert.Equal(t, "The frame rate must be greater than 0, got -1.", rerr.Msg)
	require.ErrorAs(t, sprucestl.WriteOptions{FrameRate: bad}.Validate(), &werr)
	assert.Equal(t, "WriteOptions.SpruceSTL.FrameRate", werr.Option)
	assert.Equal(t, "The Spruce STL frame rate must be greater than 0, got -1.", werr.Msg)
	require.NoError(t, sprucestl.ReadOptions{}.Validate())
	require.NoError(t, sprucestl.WriteOptions{}.Validate())
}
