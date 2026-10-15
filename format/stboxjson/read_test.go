package stboxjson_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const oneCue = `{"start": 1, "end": 2, "lines": ["Hello"]}`

func read(t *testing.T, content string, lenient bool) (*model.Subtitle, []model.ParseWarning, error) {
	t.Helper()

	return stboxjson.Read(strings.NewReader(content), model.CommonReadOptions{Lenient: lenient}, stboxjson.ReadOptions{})
}

func mustRead(t *testing.T, content string, lenient bool) *model.Subtitle {
	t.Helper()

	s, _, err := read(t, content, lenient)
	require.NoError(t, err)
	return s
}

// timedText holds the start, end and text of one cue.
type timedText struct {
	start, end time.Duration
	text       string
}

func cueTexts(s *model.Subtitle) []timedText {
	var out []timedText
	for _, c := range s.All() {
		out = append(out, timedText{c.Start, c.End, c.Text()})
	}
	return out
}

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }

// Ports testThrowsParsingExceptionWithThePathOfTheBadField of tests/Parsers/JsonParserTest.php,
// testFromArrayThrowsWithThePathOfTheBadField of tests/ArrayConversionTest.php and the library JSON rows of
// tests/JsonTypeErrorsTest.php of subtitle-toolbox-php.
func TestReadErrors(t *testing.T) {
	t.Parallel()

	deep := func(n int) string { return strings.Repeat("[", n) + strings.Repeat("]", n) }
	tests := []struct {
		name    string
		content string
		msg     string
		// wholeFile errors return in lenient mode too. Other errors skip the entry, and the lenient read keeps lenientCues.
		wholeFile   bool
		lenientCues int
	}{
		{name: "not JSON", content: `{"version": 1,`, msg: "The content is not valid JSON: unexpected end of JSON input.", wholeFile: true},
		{name: "text after the root", content: `{"version": 1, "cues": []} x`, msg: "The content is not valid JSON: invalid character 'x' after top-level value.", wholeFile: true},
		{name: "512 levels", content: `{"version": 1, "cues": [], "x": ` + deep(511) + `}`, msg: "The content is not valid JSON: Maximum stack depth exceeded.", wholeFile: true},
		{name: "list root", content: `[1, 2]`, msg: "The JSON root must be an object.", wholeFile: true},
		{name: "string root", content: `"cues"`, msg: "The JSON root must be an object.", wholeFile: true},
		{name: "number root", content: `1`, msg: "The JSON root must be an object.", wholeFile: true},
		{name: "empty list root", content: `[]`, msg: "The field version is missing.", wholeFile: true},
		{name: "no version", content: `{"cues": []}`, msg: "The field version is missing.", wholeFile: true},
		{name: "version 2", content: `{"version": 2, "cues": []}`, msg: "The field version must be 1.", wholeFile: true},
		{name: "version as string", content: `{"version": "1", "cues": []}`, msg: "The field version must be 1.", wholeFile: true},
		{name: "version as float", content: `{"version": 1.0, "cues": []}`, msg: "The field version must be 1.", wholeFile: true},
		{name: "version null", content: `{"version": null, "cues": []}`, msg: "The field version must be 1.", wholeFile: true},
		{name: "metadata as string", content: `{"version": 1, "metadata": "Bakery", "cues": []}`, msg: "The field metadata must be an object.", wholeFile: true},
		{name: "metadata value number", content: `{"version": 1, "metadata": {"title": 7}, "cues": [` + oneCue + `]}`, msg: "The field metadata.title must be a string.", lenientCues: 1},
		{name: "format data as string", content: `{"version": 1, "formatData": "x", "cues": []}`, msg: "The field formatData must be an object.", wholeFile: true},
		{name: "format data value string", content: `{"version": 1, "formatData": {"ass": "x"}, "cues": [` + oneCue + `]}`, msg: "The field formatData.ass must be an object.", lenientCues: 1},
		{name: "format data null", content: `{"version": 1, "formatData": {"ass": null}, "cues": [` + oneCue + `]}`, msg: "The field formatData.ass must be an object.", lenientCues: 1},
		{name: "SCC drop frame string", content: `{"version": 1, "formatData": {"scc": {"dropFrame": "x"}}, "cues": [` + oneCue + `]}`, msg: "The field formatData.scc.dropFrame must be a boolean.", lenientCues: 1},
		{name: "CSV without roles", content: `{"version": 1, "formatData": {"csv": {"header": null, "width": 2}}, "cues": [` + oneCue + `]}`, msg: "The field formatData.csv.roles is missing.", lenientCues: 1},
		{name: "no cues", content: `{"version": 1}`, msg: "The field cues must be a list.", wholeFile: true},
		{name: "cues null", content: `{"version": 1, "cues": null}`, msg: "The field cues must be a list.", wholeFile: true},
		{name: "cues as object", content: `{"version": 1, "cues": {"0": ` + oneCue + `}}`, msg: "The field cues must be a list.", wholeFile: true},
		{name: "cue as string", content: `{"version": 1, "cues": [` + oneCue + `, "Rain"]}`, msg: "The field cues[1] must be an object.", lenientCues: 1},
		{name: "cue as list", content: `{"version": 1, "cues": [[1, 2]]}`, msg: "The field cues[0].start must be a number."},
		{name: "start as string", content: `{"version": 1, "cues": [` + oneCue + `, {"start": "1", "end": 2, "lines": []}]}`, msg: "The field cues[1].start must be a number.", lenientCues: 1},
		{name: "start null", content: `{"version": 1, "cues": [{"start": null, "end": 1, "lines": []}]}`, msg: "The field cues[0].start must be a number."},
		{name: "start 1e400", content: `{"version": 1, "cues": [{"start": 1e400, "end": 1e400, "lines": []}]}`, msg: "The field cues[0].start must be a number."},
		{name: "no end", content: `{"version": 1, "cues": [{"start": 1, "lines": []}]}`, msg: "The field cues[0].end must be a number."},
		{name: "lines as string", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": "Rain"}]}`, msg: "The field cues[0].lines must be a list."},
		{name: "lines as object", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": {"0": "Rain"}}]}`, msg: "The field cues[0].lines must be a list."},
		{name: "no lines", content: `{"version": 1, "cues": [{"start": 1, "end": 2}]}`, msg: "The field cues[0].lines must be a list."},
		{name: "line as number", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": ["Rain", 7]}]}`, msg: "The field cues[0].lines[1] must be a string."},
		{name: "identifier as number", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "identifier": 7}]}`, msg: "The field cues[0].identifier must be a string or null."},
		{name: "alignment 10", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "alignment": 10}]}`, msg: "The field cues[0].alignment must be an integer from 1 to 9 or null."},
		{name: "alignment 0", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "alignment": 0}]}`, msg: "The field cues[0].alignment must be an integer from 1 to 9 or null."},
		{name: "alignment as float", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "alignment": 8.0}]}`, msg: "The field cues[0].alignment must be an integer from 1 to 9 or null."},
		{name: "alignment as string", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "alignment": "8"}]}`, msg: "The field cues[0].alignment must be an integer from 1 to 9 or null."},
		{name: "forced as string", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "forced": "yes"}]}`, msg: "The field cues[0].forced must be a boolean."},
		{name: "cue format data string", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "formatData": "a"}]}`, msg: "The field cues[0].formatData must be an object."},
		{name: "cue format data value", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "formatData": {"ass": "x"}}]}`, msg: "The field cues[0].formatData.ass must be an object."},
		{name: "WebVTT setting number", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "formatData": {"vtt": {"line": 0}}}]}`, msg: "The field cues[0].formatData.vtt.line must be a string."},
		{name: "SubRip coordinates", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "formatData": {"srt": {"coordinates": {"x1": 1}}}}]}`, msg: "The field cues[0].formatData.srt.coordinates.x2 is missing."},
		{name: "comments as object", content: `{"version": 1, "comments": {"a": []}, "cues": []}`, msg: "The field comments must be a list.", wholeFile: true},
		{name: "comment as string", content: `{"version": 1, "comments": ["Note"], "cues": [` + oneCue + `]}`, msg: "The field comments[0] must be an object.", lenientCues: 1},
		{name: "comment without text", content: `{"version": 1, "comments": [{"beforeCueIndex": 0}], "cues": [` + oneCue + `]}`, msg: "The field comments[0].text must be a string.", lenientCues: 1},
		{name: "negative comment index", content: `{"version": 1, "comments": [{"text": "Note", "beforeCueIndex": -1}], "cues": [` + oneCue + `]}`, msg: "The field comments[0].beforeCueIndex must be an integer of 0 or more.", lenientCues: 1},
		{name: "comment index as float", content: `{"version": 1, "comments": [{"text": "Note", "beforeCueIndex": 1.0}], "cues": [` + oneCue + `]}`, msg: "The field comments[0].beforeCueIndex must be an integer of 0 or more.", lenientCues: 1},
		{name: "start at 100000 hours", content: `{"version": 1, "cues": [` + oneCue + `, {"start": 360000000, "end": 360000001, "lines": []}]}`, msg: "The field cues[1].start is not below 100000 hours.", lenientCues: 1},
		{name: "end at 100000 hours", content: `{"version": 1, "cues": [{"start": 1, "end": 360000000.0, "lines": []}]}`, msg: "The field cues[0].end is not below 100000 hours."},
		{name: "start of -1e20", content: `{"version": 1, "cues": [{"start": -1e20, "end": 1, "lines": []}]}`, msg: "The field cues[0].start is not above -100000 hours."},
		{name: "end at -100000 hours", content: `{"version": 1, "cues": [{"start": -1, "end": -360000000, "lines": []}]}`, msg: "The field cues[0].end is not above -100000 hours."},
		{name: "word timestamp at 100000 hours", content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": ["One <100000:00:00.000>two"]}]}`, msg: `The time "100000:00:00.000" is not below 100000 hours.`},
		{name: "end before start", content: `{"version": 1, "cues": [{"start": 70, "end": 2, "lines": []}]}`, msg: "The cue ends at 2 s, before it starts at 70 s."},
		{name: "end before start with fractions", content: `{"version": 1, "cues": [{"start": 5.25, "end": 3.0, "lines": []}]}`, msg: "The cue ends at 3 s, before it starts at 5.25 s.", lenientCues: 1},
		{name: "bad base64 in a cue", content: `{"version": 1, "cues": [{"start": 0, "end": 1, "lines": [], "formatData": {"image": {"png": {"base64": "?"}}}}]}`, msg: "The field cues[0].formatData.image.png.base64 must be valid base64."},
		{name: "bad base64 in a list", content: `{"version": 1, "formatData": {"x": [{"base64": "!"}]}, "cues": [` + oneCue + `]}`, msg: "The field formatData.x[0].base64 must be valid base64.", lenientCues: 1},
		{name: "bad base64 padding", content: `{"version": 1, "formatData": {"x": {"a": {"base64": "AB="}}}, "cues": []}`, msg: "The field formatData.x.a.base64 must be valid base64."},
		{name: "bad base64 under a number key", content: `{"version": 1, "formatData": {"x": {"7": {"base64": "A"}}}, "cues": []}`, msg: "The field formatData.x[7].base64 must be valid base64."},
		{name: "base64 as the whole format data", content: `{"version": 1, "formatData": {"x": {"base64": "AAE="}}, "cues": []}`, msg: "The field formatData.x must be an object."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := read(t, tt.content, false)
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			require.ErrorIs(t, err, model.ErrParse)
			assert.Equal(t, tt.msg, perr.Msg)
			assert.Equal(t, model.JSON, perr.Format)
			assert.Zero(t, perr.Line)
			assert.Nil(t, s)
			assert.Nil(t, warnings)

			s, warnings, err = read(t, tt.content, true)
			if tt.wholeFile {
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, tt.msg, perr.Msg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.lenientCues, s.Len())
			require.Len(t, warnings, 1)
			if warnings[0].Action == model.Repaired {
				assert.Equal(t, tt.msg+" The parser swapped the times.", warnings[0].Message)
				return
			}
			assert.Equal(t, model.Skipped, warnings[0].Action)
			assert.Equal(t, tt.msg, warnings[0].Message)
		})
	}
}

func TestReadNumbers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		start, end string
		want       timedText
	}{
		{"integers", "1", "4", timedText{ms(1000), ms(4000), "Hello"}},
		{"half a millisecond rounds away from zero", "1.0005", "1.0015", timedText{ms(1001), ms(1002), "Hello"}},
		{"negative half a millisecond", "-1.0005", "-0.0004", timedText{ms(-1001), 0, "Hello"}},
		{"exponent", "1e2", "1.5E2", timedText{ms(100_000), ms(150_000), "Hello"}},
		{"last millisecond below 100000 hours", "359999998", "359999999.999", timedText{ms(359_999_998_000), ms(359_999_999_999), "Hello"}},
		{"tiny", "1e-999999999", "0.0004999", timedText{0, 0, "Hello"}},
		{"integer beyond int64", "1", "99999999999999999999e-12", timedText{ms(1000), ms(100_000_000_000), "Hello"}},
		{"negative zero", "-0", "-0.0", timedText{0, 0, "Hello"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := mustRead(t, `{"version": 1, "cues": [{"start": `+tt.start+`, "end": `+tt.end+`, "lines": ["Hello"]}]}`, false)
			assert.Equal(t, []timedText{tt.want}, cueTexts(s))
		})
	}
}

// Ports testReadsMetadataCommentsAndCueFields of tests/Parsers/JsonParserTest.php of subtitle-toolbox-php.
func TestReadsMetadataCommentsAndCueFields(t *testing.T) {
	t.Parallel()

	s := mustRead(t, `{
		"version": 1,
		"metadata": {"title": "Big Buck Bunny", "language": "en"},
		"comments": [{"text": "Translated by Jane Doe", "beforeCueIndex": 0}],
		"formatData": {},
		"cues": [
			{"start": 1.5, "end": 4.0, "lines": ["Hello", "<i>world</i>"], "identifier": "intro", "alignment": 8, "formatData": {}}
		]
	}`, false)

	assert.Equal(t, map[string]string{"title": "Big Buck Bunny", "language": "en"}, s.AllMetadata())
	assert.Equal(t, []model.Comment{{Text: "Translated by Jane Doe", BeforeCue: 0}}, s.Comments())
	assert.Equal(t, []model.Cue{{
		Start: ms(1500), End: ms(4000), Lines: []string{"Hello", "<i>world</i>"}, ID: "intro", Alignment: model.TopCenter,
	}}, s.Cues())
}

// Ports testFromArrayFillsMissingOptionalFields of tests/ArrayConversionTest.php of subtitle-toolbox-php.
func TestReadFillsMissingOptionalFields(t *testing.T) {
	t.Parallel()

	for _, content := range []string{
		`{"version": 1, "cues": [{"start": 1, "end": 2, "lines": ["Rain"]}]}`,
		`{"version": 1, "metadata": null, "comments": null, "formatData": null, "cues": [
			{"start": 1, "end": 2, "lines": ["Rain"], "identifier": null, "alignment": null, "forced": null, "formatData": null}]}`,
		`{"version": 1, "metadata": [], "comments": [], "formatData": [], "cues": [
			{"start": 1, "end": 2, "lines": ["Rain"], "identifier": "", "alignment": 2, "forced": false, "formatData": []}]}`,
	} {
		s := mustRead(t, content, false)
		assert.Empty(t, s.AllMetadata())
		assert.Empty(t, s.Comments())
		assert.Empty(t, s.FormatDataKeys())
		assert.Equal(t, []model.Cue{{Start: ms(1000), End: ms(2000), Lines: []string{"Rain"}}}, s.Cues())
	}
}

func TestReadLists(t *testing.T) {
	t.Parallel()

	s := mustRead(t, `{"version": 1, "metadata": ["zero", "one"], "formatData": {"mpsub": ["a", "b"], "x": {"list": [], "object": {}}},
		"cues": [{"start": 1, "end": 2, "lines": [], "formatData": [{"a": 1}]}]}`, false)

	assert.Equal(t, map[string]string{"0": "zero", "1": "one"}, s.AllMetadata())
	mpsub, _ := s.FormatData("mpsub")
	assert.Equal(t, model.NewData(model.Field{Key: "0", Value: "a"}, model.Field{Key: "1", Value: "b"}), mpsub)
	x, _ := s.FormatData("x")
	assert.Equal(t, model.NewData(model.Field{Key: "list", Value: []any{}}, model.Field{Key: "object", Value: model.NewData()}), x)
	c, _ := s.Cue(0)
	zero, _ := c.FormatData("0")
	assert.Equal(t, model.NewData(model.Field{Key: "a", Value: int64(1)}), zero)
}

func TestReadFormatDataValues(t *testing.T) {
	t.Parallel()

	s := mustRead(t, `{"version": 1, "formatData": {"x": {"int": 4, "float": 4.0, "exp": 1E2, "big": 99999999999999999999,
		"inf": 1e400, "null": null, "true": true, "text": "a/b", "dup": 1, "dup": 2}}, "cues": []}`, false)

	x, ok := s.FormatData("x")
	require.True(t, ok)
	assert.Equal(t, model.NewData(
		model.Field{Key: "int", Value: int64(4)},
		model.Field{Key: "float", Value: 4.0},
		model.Field{Key: "exp", Value: 100.0},
		model.Field{Key: "big", Value: 1e20},
		model.Field{Key: "inf", Value: math.Inf(1)},
		model.Field{Key: "null", Value: nil},
		model.Field{Key: "true", Value: true},
		model.Field{Key: "text", Value: "a/b"},
		model.Field{Key: "dup", Value: int64(2)},
	), x)
}

// Ports testDecodesBase64ObjectsInFormatData and testKeepsObjectsWithMoreKeysThanBase64 of
// tests/Parsers/JsonParserTest.php of subtitle-toolbox-php.
func TestReadDecodesBase64(t *testing.T) {
	t.Parallel()

	s := mustRead(t, "\ufeff"+`{"version": 1, "formatData": {"x": {"list": [{"base64": "AAE="}], "more": {"base64": "AAE=", "note": "y"},
		"spaces": {"base64": " AA\tE=\r\n"}, "unpadded": {"base64": "AAE"}, "empty": {"base64": ""}}},
		"cues": [{"start": 1, "end": 2, "lines": [], "formatData": {"image": {"png": {"base64": "iVBORw=="}}}}]}`, false)

	x, _ := s.FormatData("x")
	assert.Equal(t, model.NewData(
		model.Field{Key: "list", Value: []any{"\x00\x01"}},
		model.Field{Key: "more", Value: model.NewData(model.Field{Key: "base64", Value: "AAE="}, model.Field{Key: "note", Value: "y"})},
		model.Field{Key: "spaces", Value: "\x00\x01"},
		model.Field{Key: "unpadded", Value: "\x00\x01"},
		model.Field{Key: "empty", Value: ""},
	), x)
	c, _ := s.Cue(0)
	image, _ := c.FormatData(model.ImageKey)
	assert.Equal(t, model.NewData(model.Field{Key: "png", Value: "\x89PNG"}), image)
}

func TestReadBadBase64InLenientMode(t *testing.T) {
	t.Parallel()

	s := mustRead(t, `{"version": 1, "formatData": {"x": {"a": {"base64": "!"}}, "y": {"a": 1}},
		"cues": [{"start": 1, "end": 2, "lines": ["Bad"], "formatData": {"image": {"png": {"base64": "!"}}}}, `+oneCue+`]}`, true)

	assert.Equal(t, []string{"y"}, s.FormatDataKeys())
	assert.Equal(t, []timedText{{ms(1000), ms(2000), "Hello"}}, cueTexts(s))
}

// Ports testReadsAnInvalidUtf8ByteAsTheReplacementCharacter of tests/Parsers/JsonFormatRulesTest.php of subtitle-toolbox-php.
func TestReadsAnInvalidUTF8ByteAsTheReplacementCharacter(t *testing.T) {
	t.Parallel()

	for _, lenient := range []bool{false, true} {
		s := mustRead(t, `{"version": 1, "cues": [{"start": 0, "end": 1, "lines": ["B`+"\xff"+`d"]}]}`, lenient)
		c, _ := s.Cue(0)
		assert.Equal(t, "B\ufffdd", c.Text())
	}
}

func TestReadCleansLines(t *testing.T) {
	t.Parallel()

	s := mustRead(t, `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": ["  Two  spaces ", "", "\tTab"]}]}`, false)
	c, _ := s.Cue(0)
	assert.Equal(t, []string{"Two spaces", "Tab"}, c.Lines)
}

func TestReadSwapsShortReversedTimesInLenientMode(t *testing.T) {
	t.Parallel()

	content := `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": ["Welcome on board."]}, {"start": 5, "end": 3, "lines": ["Swapped"]},
		{"start": 40, "end": 10, "lines": ["30 s"]}, {"start": 70.001, "end": 40, "lines": ["30.001 s"]}, {"start": 0, "end": 0, "lines": ["Same"]}]}`
	s := mustRead(t, content, true)
	assert.Equal(t, []timedText{
		{0, 0, "Same"},
		{ms(1000), ms(2000), "Welcome on board."},
		{ms(3000), ms(5000), "Swapped"},
		{ms(10_000), ms(40_000), "30 s"},
	}, cueTexts(s))
}

func TestReadKeepsNegativeReversedTimes(t *testing.T) {
	t.Parallel()

	s := mustRead(t, `{"version": 1, "cues": [{"start": 5, "end": -1, "lines": ["Negative end"]}]}`, false)
	assert.Equal(t, []timedText{{ms(5000), ms(-1000), "Negative end"}}, cueTexts(s))
}

func TestReadKeepsCommentsBeforeTheirCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		lenient  bool
		want     []timedText
		comments []model.Comment
	}{
		{
			name: "cues in order",
			content: `{"version": 1, "comments": [{"text": "Before B", "beforeCueIndex": 1}, {"text": "End", "beforeCueIndex": 9}],
				"cues": [{"start": 1, "end": 2, "lines": ["A"]}, {"start": 3, "end": 4, "lines": ["B"]}]}`,
			want:     []timedText{{ms(1000), ms(2000), "A"}, {ms(3000), ms(4000), "B"}},
			comments: []model.Comment{{Text: "Before B", BeforeCue: 1}, {Text: "End", BeforeCue: 9}},
		},
		{
			name: "cues out of order",
			content: `{"version": 1, "comments": [{"text": "Before B", "beforeCueIndex": 0}, {"text": "Before A", "beforeCueIndex": 1},
				{"text": "End", "beforeCueIndex": 2}],
				"cues": [{"start": 3, "end": 4, "lines": ["B"]}, {"start": 1, "end": 2, "lines": ["A"]}]}`,
			want:     []timedText{{ms(1000), ms(2000), "A"}, {ms(3000), ms(4000), "B"}},
			comments: []model.Comment{{Text: "Before A", BeforeCue: 0}, {Text: "Before B", BeforeCue: 1}, {Text: "End", BeforeCue: 2}},
		},
		{
			name: "comment after a skipped cue",
			content: `{"version": 1, "metadata": {"title": "Station announcements"}, "comments": [{"text": "Platform changes", "beforeCueIndex": 2}],
				"cues": [{"start": 1, "end": 3, "lines": ["The train to the coast is late."]}, {"start": 4, "lines": ["It leaves from platform two."]},
				{"start": 7, "end": 9, "lines": ["The buffet car is closed."]}, {"start": 10, "end": 12, "lines": ["Mind the step.", 5]}]}`,
			lenient:  true,
			want:     []timedText{{ms(1000), ms(3000), "The train to the coast is late."}, {ms(7000), ms(9000), "The buffet car is closed."}},
			comments: []model.Comment{{Text: "Platform changes", BeforeCue: 1}},
		},
		{
			name: "comment before a skipped cue",
			content: `{"version": 1, "comments": [{"text": "Before the bad cue", "beforeCueIndex": 1}],
				"cues": [{"start": 1, "end": 2, "lines": ["A"]}, {"start": "x", "end": 4, "lines": ["Bad"]}, {"start": 5, "end": 6, "lines": ["C"]}]}`,
			lenient:  true,
			want:     []timedText{{ms(1000), ms(2000), "A"}, {ms(5000), ms(6000), "C"}},
			comments: []model.Comment{{Text: "Before the bad cue", BeforeCue: 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := mustRead(t, tt.content, tt.lenient)
			assert.Equal(t, tt.want, cueTexts(s))
			assert.Equal(t, tt.comments, s.Comments())
		})
	}
}

func TestReadAddsTheAlignmentOf3xFormatData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		alignment string
		data      string
		want      *model.Data
	}{
		{"vtt without alignment", "8", `{"vtt": {"line": "0"}}`, model.NewData(model.Field{Key: "line", Value: "0"}, model.Field{Key: "alignment", Value: int64(8)})},
		{
			"ttml without alignment", "7", `{"ttml": {"attributes": {"region": "r1"}}}`,
			model.NewData(model.Field{Key: "attributes", Value: model.NewData(model.Field{Key: "region", Value: "r1"})}, model.Field{Key: "alignment", Value: int64(7)}),
		},
		{"vtt with alignment", "8", `{"vtt": {"line": "0", "alignment": 2}}`, model.NewData(model.Field{Key: "line", Value: "0"}, model.Field{Key: "alignment", Value: int64(2)})},
		{"bottom center", "null", `{"vtt": {"line": "0"}}`, model.NewData(model.Field{Key: "line", Value: "0"})},
		{"empty data", "8", `{"vtt": {}}`, nil},
		{"other format", "8", `{"ass": {"text": "x"}}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := mustRead(t, `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "alignment": `+tt.alignment+`, "formatData": `+tt.data+`}]}`, false)
			c, _ := s.Cue(0)
			var got *model.Data
			for _, key := range []string{"vtt", "ttml"} {
				if d, ok := c.FormatData(key); ok {
					got = d
				}
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadEmptyContent(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", "\ufeff \n\t"} {
		s := mustRead(t, content, false)
		assert.Zero(t, s.Len())
	}
}
