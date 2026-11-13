package youtube_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const json3Hello = `{"events": [{"tStartMs": 1200, "dDurationMs": 2300, "segs": [{"utf8": "Hello"}, {"utf8": " world", "tOffsetMs": 400}]}]}`

// Ports testEveryShapeGivesTheSameCue and testWordTimestampsOption of
// tests/Parsers/YouTubeTimedTextParserTest.php of subtitle-toolbox-php.
func TestReadJSON3WordTimes(t *testing.T) {
	t.Parallel()

	for name, opts := range map[string]youtube.ReadOptions{"default": {}, "words": withWords} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, _ := read(t, json3Hello, opts).Cue(0)
			assert.Equal(t, [][2]int{{1200, 3500}}, [][2]int{{int(c.Start.Milliseconds()), int(c.End.Milliseconds())}})
			assert.Equal(t, []string{"Hello world"}, c.Lines)
			if opts.WordTimestamps {
				assert.Equal(t, []model.Word{{Text: "Hello", Start: ms(1200), End: ms(1600)}, {Text: "world", Start: ms(1600), End: ms(3500)}}, c.Words)
			} else {
				assert.Empty(t, c.Words)
			}
		})
	}
}

// Ports testSkipsAppendEventsAndEndsRollingCuesAtTheNextCueOfTheirWindow.
func TestReadJSON3WindowsAndAppends(t *testing.T) {
	t.Parallel()

	content := `{"events": [
		{"tStartMs": 0, "dDurationMs": 9000, "id": 1, "wpWinPosId": 1},
		{"tStartMs": 100, "dDurationMs": 4000, "wWinId": 1, "segs": [{"utf8": "one"}]},
		{"tStartMs": 2000, "dDurationMs": 2100, "wWinId": 1, "aAppend": 1, "segs": [{"utf8": "\n"}]},
		{"tStartMs": 2010, "dDurationMs": 4000, "wWinId": 1, "segs": [{"utf8": "two"}]},
		{"tStartMs": 3000, "dDurationMs": 1000, "segs": [{"utf8": "sign"}]},
		{"tStartMs": 4000, "dDurationMs": 5000, "wWinId": "1", "segs": [{"utf8": "text window"}]}
	]}`
	s := read(t, content, youtube.ReadOptions{})
	assert.Equal(t, [][2]int{{100, 2010}, {2010, 6010}, {3000, 4000}, {4000, 9000}}, times(s))
	file, _ := s.FormatData("youtube")
	windows, _ := file.List("windows")
	assert.Len(t, windows, 1)
	c, _ := s.Cue(1)
	data, _ := c.FormatData("youtube")
	appends, _ := data.List("appends")
	assert.Len(t, appends, 1)
}

func TestReadJSON3ReadsATextThatIsNoStringAsEmpty(t *testing.T) {
	t.Parallel()

	c, _ := read(t, `{"events": [{"tStartMs": 0, "dDurationMs": 1, "segs": [{"utf8": 5}, {"utf8": "a"}]}]}`, youtube.ReadOptions{}).Cue(0)
	assert.Equal(t, []string{"a"}, c.Lines)
}

func TestReadJSON3TextIsNotDecoded(t *testing.T) {
	t.Parallel()

	c, _ := read(t, `{"events": [{"tStartMs": 0, "dDurationMs": 1, "segs": [{"utf8": "&amp; <b>"}]}]}`, youtube.ReadOptions{}).Cue(0)
	assert.Equal(t, []string{"&amp;amp; &lt;b&gt;"}, c.Lines)
}

// Ports testJson3WindowPositionsAndPens.
func TestReadJSON3PensAndWindowPositions(t *testing.T) {
	t.Parallel()

	content := `{"wireMagic": "pb3", "pens": [{}, {"bAttr": 1, "fcForeColor": 65280, "foForeAlpha": 254}],
		"wpWinPositions": [{}, {"apPoint": 8, "ahHorPos": 100, "avVerPos": 100}],
		"events": [{"tStartMs": 0, "dDurationMs": 1000, "wpWinPosId": 1, "segs": [{"utf8": "green", "pPenId": 1}, {"utf8": " text"}]}]}`
	s := read(t, content, youtube.ReadOptions{})
	c, _ := s.Cue(0)
	assert.Equal(t, model.BottomRight, c.Position.Alignment)
	assert.Equal(t, []string{`<font color="#00ff00"><b>green</b></font> text`}, c.Lines)
	data, _ := c.FormatData("youtube")
	assert.Equal(t, []string{"tStartMs", "dDurationMs", "wpWinPosId", "segments"}, data.Keys())
	file, _ := s.FormatData("youtube")
	assert.Equal(t, []string{"format", "wireMagic", "pens", "wpWinPositions"}, file.Keys())
}

// Ports testWordTimestampsKeepLineBreaksAndSkipCuesWithoutWordTimes.
func TestReadJSON3WordTimesKeepLineBreaks(t *testing.T) {
	t.Parallel()

	content := `{"events": [{"tStartMs": 0, "dDurationMs": 3000, "segs": [{"utf8": "one"}, {"utf8": "\ntwo", "tOffsetMs": 1500}]},
		{"tStartMs": 3000, "dDurationMs": 1000, "segs": [{"utf8": "[Music]"}]}]}`
	s := read(t, content, withWords)
	c, _ := s.Cue(0)
	assert.Equal(t, []string{"one", "two"}, c.Lines)
	assert.Equal(t, []model.Word{{Text: "one", Start: 0, End: ms(1500)}, {Text: "two", Start: ms(1500), End: ms(3000)}}, c.Words)
	c, _ = s.Cue(1)
	assert.Equal(t, []string{"[Music]"}, c.Lines)
	assert.Empty(t, c.Words)
}

func TestReadJSON3Errors(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, content, want string }{
		{"not JSON", `{"events": [`, "The content is not valid JSON:"},
		{"no events", `{"pens": []}`, `The JSON has no "events" list.`},
		{"events as an object", `{"events": {"0": {"tStartMs": 0}}}`, `The JSON has no "events" list.`},
		{"start as text", `{"events": [{"tStartMs": "0"}]}`, "The field events[0].tStartMs must be a number."},
		{"an event that is no object", `{"events": [{"tStartMs": 0}, 5]}`, "The field events[1].tStartMs must be a number."},
		{"negative start", `{"events": [{"tStartMs": -1}]}`, "The field events[0].tStartMs must be a number."},
		{"start over the limit", `{"events": [{"tStartMs": 360000000000}]}`, "The field events[0].tStartMs is not below 100000 hours."},
		{"duration as text", `{"events": [{"tStartMs": 0, "dDurationMs": "1"}]}`, "The field events[0].dDurationMs must be a number."},
		{"end over the limit", `{"events": [{"tStartMs": 359999999999, "dDurationMs": 2}]}`, "The field events[0].dDurationMs is not below 100000 hours."},
		{"offset as text", `{"events": [{"tStartMs": 0, "segs": [{"utf8": "a", "tOffsetMs": "5"}]}]}`, "The field events[0].segs[0].tOffsetMs must be a number."},
		{"offset over the limit", `{"events": [{"tStartMs": 359999999999, "segs": [{"utf8": "a", "tOffsetMs": 2}]}]}`, "The field events[0].segs[0].tOffsetMs is not below 100000 hours."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := youtube.Read(strings.NewReader(tt.content), model.CommonReadOptions{}, youtube.ReadOptions{})
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.True(t, strings.HasPrefix(perr.Msg, tt.want), perr.Msg)
		})
	}
}

// Ports testLenientModeSkipsBrokenEvents.
func TestReadJSON3LenientSkipsABrokenEvent(t *testing.T) {
	t.Parallel()

	content := `{"events": [{"tStartMs": "0", "segs": [{"utf8": "a"}]}, {"tStartMs": 1000, "dDurationMs": 1000, "segs": [{"utf8": "b"}]}]}`
	s, warnings, err := youtube.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, youtube.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, s.Len())
	require.Len(t, warnings, 1)
	assert.Equal(t, "The field events[0].tStartMs must be a number.", warnings[0].Message)
	assert.Equal(t, model.NoLine, warnings[0].Line)
	assert.Equal(t, 0, warnings[0].Block)
	assert.Equal(t, []string{`{"tStartMs":"0","segs":[{"utf8":"a"}]}`}, warnings[0].Lines)
}

func TestReadJSON3ReadsABadByteAsReplacementInStrictMode(t *testing.T) {
	t.Parallel()

	c, _ := read(t, "{\"events\": [{\"tStartMs\": 0, \"segs\": [{\"utf8\": \"caf\xe9\"}]}]}", youtube.ReadOptions{}).Cue(0)
	assert.Equal(t, []string{"caf" + "\ufffd"}, c.Lines)
}
