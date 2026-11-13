package youtube_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func decodeJSON(t *testing.T, s string) orderedjson.Value {
	t.Helper()

	v, err := orderedjson.Decode([]byte(s))
	require.NoError(t, err)
	return v
}

// Ports testARoundTripGivesTheSource of tests/Formatters/YouTubeTimedTextFormatterTest.php of subtitle-toolbox-php for
// json3: the same data in the same key order, without the indents of YouTube.
func TestWriteJSON3RoundTrip(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"manual.en.json3", "auto.en.json3"} {
		for name, opts := range map[string]youtube.ReadOptions{"default": {}, "words": withWords} {
			t.Run(file+"/"+name, func(t *testing.T) {
				t.Parallel()

				s, src := readFile(t, file, opts)
				out := write(t, s)
				assert.True(t, orderedjson.Equal(decodeJSON(t, src), decodeJSON(t, out)), out)
				assert.NotContains(t, out, "\n")
			})
		}
	}
}

// Ports testAJson3EditGetsPensAndAWindowPositionAsInSrv3.
func TestWriteJSON3EditGetsPensAndAWindowPosition(t *testing.T) {
	t.Parallel()

	s, _ := readFile(t, "manual.en.json3", youtube.ReadOptions{})
	editCue(t, s, 0, func(c *model.Cue) {
		c.SetText(`Welcome to the <font color="#ff0000"><b>bakery</b></font> tour.`)
		c.Position.Alignment = model.TopCenter
	})
	out := write(t, s)
	assert.Contains(t, out, `"pens":[{},{"bAttr":1,"fcForeColor":16711680}]`)
	assert.Contains(t, out, `"wpWinPositions":[{},{"apPoint":1,"ahHorPos":50,"avVerPos":0}]`)
	assert.Contains(t, out, `{"tStartMs":1200,"dDurationMs":2300,"wpWinPosId":1,"segs":[{"utf8":"Welcome to the "},{"utf8":"bakery","pPenId":1},{"utf8":" tour."}]}`)
}

// Ports testCuesFromAnotherFormatGiveJson3WhenTheFormatDataSaysSo.
func TestWriteJSON3FromAnotherFormat(t *testing.T) {
	t.Parallel()

	c := model.Cue{
		Start: ms(1000), End: ms(2000), Lines: []string{"<i>Hi</i> there"},
		Words: []model.Word{{Text: "Hi", Start: ms(1000), End: ms(1500)}, {Text: "there", Start: ms(1500), End: ms(2000)}},
	}
	s := fromAnotherFormat(c)
	require.NoError(t, s.SetFormatData("youtube", model.NewData(model.Field{Key: "format", Value: "json3"})))
	want := `{"wireMagic":"pb3","pens":[{},{"iAttr":1}],"wsWinStyles":[{}],"wpWinPositions":[{}],` +
		`"events":[{"tStartMs":1000,"dDurationMs":1000,"segs":[{"utf8":"Hi","pPenId":1},{"utf8":" there","tOffsetMs":500}]}]}`
	assert.Equal(t, want, write(t, s))
}

func TestWriteJSON3KeepsWindowsAndAppendsAndMovesThem(t *testing.T) {
	t.Parallel()

	content := `{"wireMagic": "pb3", "events": [
		{"tStartMs": 0, "dDurationMs": 9000, "id": 1},
		{"tStartMs": 100, "dDurationMs": 4000, "wWinId": 1, "segs": [{"utf8": "one"}]},
		{"tStartMs": 2000, "dDurationMs": 2100, "wWinId": 1, "aAppend": 1, "segs": [{"utf8": "\n"}]},
		{"tStartMs": 2010, "dDurationMs": 4000, "wWinId": 1, "segs": [{"utf8": "two"}]},
		{"tStartMs": 9000, "aAppend": 1, "segs": [{"utf8": "\n"}]}
	]}`
	s := read(t, content, youtube.ReadOptions{})
	require.NoError(t, s.Shift(ms(500)))
	want := `{"wireMagic":"pb3","pens":[{}],"wsWinStyles":[{}],"wpWinPositions":[{}],"events":[` +
		`{"tStartMs":0,"dDurationMs":9000,"id":1},` +
		`{"tStartMs":600,"dDurationMs":4000,"wWinId":1,"segs":[{"utf8":"one"}]},` +
		`{"tStartMs":2500,"dDurationMs":2100,"wWinId":1,"aAppend":1,"segs":[{"utf8":"\n"}]},` +
		`{"tStartMs":2510,"dDurationMs":4000,"wWinId":1,"segs":[{"utf8":"two"}]},` +
		`{"tStartMs":9000,"aAppend":1,"segs":[{"utf8":"\n"}]}]}`
	assert.Equal(t, want, write(t, s))
}

func TestWriteJSON3RejectsInvalidUTF8(t *testing.T) {
	t.Parallel()

	s := read(t, json3Hello, youtube.ReadOptions{})
	editCue(t, s, 0, func(c *model.Cue) { c.Lines = []string{"caf\xe9"} })
	err := youtube.Write(&bytes.Buffer{}, s, model.CommonWriteOptions{}, youtube.WriteOptions{})
	require.ErrorIs(t, err, model.ErrInvalidUTF8)
}
