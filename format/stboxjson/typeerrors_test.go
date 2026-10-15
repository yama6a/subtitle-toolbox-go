package stboxjson_test

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// withCueField returns the JSON of a real file with the field key of cue 0 set to the JSON value.
func withCueField(t *testing.T, file, key, value string) string {
	t.Helper()

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "json/real/"+file), &doc))
	var cues []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["cues"], &cues))
	cues[0][key] = json.RawMessage(value)
	var err error
	doc["cues"], err = json.Marshal(cues)
	require.NoError(t, err)
	out, err := json.Marshal(doc)
	require.NoError(t, err)
	return string(out)
}

func cueCount(t *testing.T, file string) int {
	t.Helper()

	var doc struct {
		Cues []json.RawMessage `json:"cues"`
	}
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "json/real/"+file), &doc))
	return len(doc.Cues)
}

func strictMessage(t *testing.T, content string) string {
	t.Helper()

	_, _, err := read(t, content, false)
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	return perr.Msg
}

// Ports the library JSON row of testInfiniteTimeThrowsInStrictModeAndSkipsInLenientMode of
// tests/JsonTypeErrorsTest.php of subtitle-toolbox-php.
func TestReadInfiniteTime(t *testing.T) {
	t.Parallel()

	content := withCueField(t, "own_image_cues.json", "start", "1e400")
	assert.Equal(t, "The field cues[0].start must be a number.", strictMessage(t, content))

	s, warnings, err := read(t, content, true)
	require.NoError(t, err)
	require.NotEmpty(t, warnings)
	assert.Equal(t, "The field cues[0].start must be a number.", warnings[0].Message)
	for _, c := range s.All() {
		assert.Less(t, math.Abs(c.Start.Seconds()), 360_000_000.0)
		assert.Less(t, math.Abs(c.End.Seconds()), 360_000_000.0)
	}
}

// Ports testBadFileFormatDataThrowsInStrictModeAndIsDroppedInLenientMode of tests/JsonTypeErrorsTest.php of
// subtitle-toolbox-php. The PHP test also writes each format, which needs the writers of the later format issues.
func TestReadBadFileFormatData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, key, data, msg string
	}{
		{"ASS section order string", "ass", `{"ass": {"sectionOrder": "x"}}`, "The field formatData.ass.sectionOrder must be a list or an object."},
		{"ASS script info string", "ass", `{"ass": {"scriptInfo": "x"}}`, "The field formatData.ass.scriptInfo must be a list or an object."},
		{"ASS script type number", "ass", `{"ass": {"scriptInfo": {"ScriptType": 4}}}`, "The field formatData.ass.scriptInfo.ScriptType must be a string."},
		{"ASS section lines string", "ass", `{"ass": {"sections": {"Fonts": "x"}}}`, "The field formatData.ass.sections.Fonts must be a list or an object."},
		{"SCC drop frame string", "scc", `{"scc": {"dropFrame": "x"}}`, "The field formatData.scc.dropFrame must be a boolean."},
		{"SAMI style number", "sami", `{"sami": {"style": 7}}`, "The field formatData.sami.style must be a string."},
		{"MicroDVD frame rate", "microdvd", `{"microdvd": {"frameRate": "x"}}`, "The field formatData.microdvd.frameRate must be a finite number."},
		{"TTML namespace number", "ttml", `{"ttml": {"namespace": 7}}`, "The field formatData.ttml.namespace must be a string."},
		{"TTML attributes list", "ttml", `{"ttml": {"attributes": ["x"]}}`, "The field formatData.ttml.attributes must be an object whose keys are names, not numbers."},
		{"SubViewer header string", "subviewer", `{"subviewer": {"header": "x"}}`, "The field formatData.subviewer.header must be a list or an object."},
		{"EBU STL GSI string", "stl", `{"stl": {"gsi": "x"}}`, "The field formatData.stl.gsi must be a list or an object."},
		{
			"EBU STL comment block", "stl", `{"stl": {"comments": [{"text": "x", "blocks": ["zz"]}]}}`,
			"The field formatData.stl.comments[0].blocks[0] must be a TTI block of 256 hexadecimal digits.",
		},
		{
			"CSV width too large", "csv", `{"csv": {"header": null, "roles": {"start": 0}, "width": 1000000000}}`,
			"The field formatData.csv.width must be an integer from 0 to 1000.",
		},
		{
			"CSV unknown role", "csv", `{"csv": {"header": null, "roles": {"start": 0, "x": 1}, "width": 2}}`,
			"The field formatData.csv.roles must hold only the keys identifier, start, end, duration, speaker, text.",
		},
		{"CSV without roles", "csv", `{"csv": {"header": null, "width": 2}}`, "The field formatData.csv.roles is missing."},
		{
			"CSV pipe delimiter", "csv", `{"csv": {"delimiter": "|", "header": null, "roles": {"start": 0}, "width": 1}}`,
			`The field formatData.csv.delimiter must be one of ",", ";", "\t".`,
		},
		{
			"CSV unknown time format", "csv", `{"csv": {"header": null, "roles": {"start": 0}, "width": 1, "timeFormat": "hh:mm:ss;fff"}}`,
			`The field formatData.csv.timeFormat must be one of "seconds", "hh:mm:ss.mmm", "hh:mm:ss,mmm", "hh:mm:ss:ff", "milliseconds".`,
		},
		{"LRC tag number", "lrc", `{"lrc": {"idTags": {"ar": 5}}}`, "The field formatData.lrc.idTags.ar must be a string."},
		{"WebVTT header lines", "vtt", `{"vtt": {"headerLines": [5]}}`, "The field formatData.vtt.headerLines[0] must be a string."},
		{"MPSub value list", "mpsub", `{"mpsub": {"NOTE": ["x"]}}`, "The field formatData.mpsub.NOTE must be a string."},
		{"iTT frame rate number", "itt", `{"itt": {"frameRate": 25}}`, "The field formatData.itt.frameRate must be a string."},
		{
			"FFmetadata stream tags", "ffmeta-chapters", `{"ffmeta-chapters": {"streams": ["x"]}}`,
			"The field formatData.ffmeta-chapters.streams[0] must be a list or an object.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := `{"version": 1, "metadata": [], "formatData": ` + tt.data + `, "cues": [{"start": 1, "end": 2, "lines": ["Hello"]}]}`
			assert.Equal(t, tt.msg, strictMessage(t, content))

			s, warnings, err := read(t, content, true)
			require.NoError(t, err)
			var line bytes.Buffer
			require.NoError(t, json.Compact(&line, []byte(tt.data)))
			assert.Equal(t, []model.ParseWarning{
				{Message: tt.msg, Block: model.NoBlock, Lines: []string{line.String()}, Action: model.Skipped},
			}, warnings)
			_, ok := s.FormatData(tt.key)
			assert.False(t, ok)
			assert.Equal(t, 1, s.Len())
		})
	}
}

// Ports testBadCueFormatDataThrowsInStrictModeAndSkipsTheCueInLenientMode of tests/JsonTypeErrorsTest.php of
// subtitle-toolbox-php.
func TestReadBadCueFormatData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, data, msg string
	}{
		{"ASS fields string", `{"ass": {"fields": "x"}}`, "The field cues[0].formatData.ass.fields must be an object."},
		{"ASS field number", `{"ass": {"fields": {"Name": 7}}}`, "The field cues[0].formatData.ass.fields.Name must be a string."},
		{"SAMI paragraph", `{"sami": {"paragraphs": [{"attributes": [], "html": 5}]}}`, "The field cues[0].formatData.sami.paragraphs[0].html must be a string."},
		{"MicroDVD line without codes", `{"microdvd": {"lines": [{"color": null, "tags": []}]}}`, "The field cues[0].formatData.microdvd.lines[0].codes is missing."},
		{"TTML attribute number", `{"ttml": {"attributes": {"region": []}}}`, "The field cues[0].formatData.ttml.attributes.region must be a string."},
		{"EBU STL group string", `{"stl": {"subtitleGroupNumber": "1"}}`, "The field cues[0].formatData.stl.subtitleGroupNumber must be an integer."},
		{"SubRip coordinates", `{"srt": {"coordinates": {"x1": 1}}}`, "The field cues[0].formatData.srt.coordinates.x2 is missing."},
		{
			"FFmetadata time base", `{"ffmeta-chapters": {"timeBase": "0/1000"}}`,
			`The field cues[0].formatData.ffmeta-chapters.timeBase must be a time base such as "1/1000".`,
		},
		{"image size string", `{"image": {"width": "2"}}`, "The field cues[0].formatData.image.width must be an integer."},
		{"WebVTT setting number", `{"vtt": {"line": 0}}`, "The field cues[0].formatData.vtt.line must be a string."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := withCueField(t, "own_aegisub.json", "formatData", tt.data)
			assert.Equal(t, tt.msg, strictMessage(t, content))

			s, warnings, err := read(t, content, true)
			require.NoError(t, err)
			require.NotEmpty(t, warnings)
			assert.Equal(t, tt.msg, warnings[0].Message)
			assert.Equal(t, 0, warnings[0].Block)
			assert.Equal(t, cueCount(t, "own_aegisub.json")-1, s.Len())
		})
	}
}
