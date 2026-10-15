package stboxjson_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports the library JSON rows of the damagedFiles table of tests/Parsers/LenientParsingTest.php,
// testJsonMovesTheCommentsToTheCueNumbersAfterTheSkip of tests/Parsers/LenientOtherFormatsTest.php, and
// testLenientJsonDropsABadMetadataFieldAndABadComment and testBadFileFormatDataThrowsInStrictModeAndIsDroppedInLenientMode
// of tests/JsonTypeErrorsTest.php of subtitle-toolbox-php.
func TestLenientWarnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []model.ParseWarning
	}{
		{
			name: "cue without end and a line that is not a string",
			content: `{"version": 1, "metadata": {"title": "Station announcements"},
				"comments": [{"text": "Platform changes", "beforeCueIndex": 2}],
				"cues": [
					{"start": 1, "end": 3, "lines": ["The train to the coast is late."]},
					{"start": 4, "lines": ["It leaves from platform two."]},
					{"start": 7, "end": 9, "lines": ["The buffet car is closed."]},
					{"start": 10, "end": 12, "lines": ["Mind the step.", 5]}
				]}`,
			want: []model.ParseWarning{
				{Message: "The field cues[1].end must be a number.", Block: 1, Lines: []string{`{"start":4,"lines":["It leaves from platform two."]}`}, Action: model.Skipped},
				{Message: "The field cues[3].lines[1] must be a string.", Block: 3, Lines: []string{`{"start":10,"end":12,"lines":["Mind the step.",5]}`}, Action: model.Skipped},
			},
		},
		{
			name: "cues that end before they start",
			content: `{"version": 1, "metadata": {"title": "Harbour tour"}, "cues": [
					{"start": 1, "end": 2, "lines": ["Welcome on board."]},
					{"start": 5, "end": 3, "lines": ["The tour takes one hour."]},
					{"start": 70, "end": 2, "lines": ["Life jackets are under the seats."]},
					{"start": 10, "end": 12, "lines": ["Enjoy the view."]}
				]}`,
			want: []model.ParseWarning{
				{
					Message: "The cue ends at 3 s, before it starts at 5 s. The parser swapped the times.", Block: 1,
					Lines: []string{`{"start":5,"end":3,"lines":["The tour takes one hour."]}`}, Action: model.Repaired,
				},
				{
					Message: "The cue ends at 2 s, before it starts at 70 s.", Block: 2,
					Lines: []string{`{"start":70,"end":2,"lines":["Life jackets are under the seats."]}`}, Action: model.Skipped,
				},
			},
		},
		{
			name: "start of 1e20 seconds",
			content: `{"version": 1, "cues": [
					{"start": 1, "end": 3, "lines": ["The pool opens at seven."]},
					{"start": 1e20, "end": 1e20, "lines": ["Towels are at the desk."]},
					{"start": 7, "end": 9, "lines": ["No running, please."]}
				]}`,
			want: []model.ParseWarning{
				{
					Message: "The field cues[1].start is not below 100000 hours.", Block: 1,
					Lines: []string{`{"start":100000000000000000000.0,"end":100000000000000000000.0,"lines":["Towels are at the desk."]}`}, Action: model.Skipped,
				},
			},
		},
		{
			name: "bad metadata and a bad comment",
			content: `{"version": 1, "metadata": {"title": 5, "language": "en"},
				"comments": [{"text": 5, "beforeCueIndex": 0}, {"text": "Kept", "beforeCueIndex": 1}],
				"cues": [{"start": 1, "end": 2, "lines": ["Hello"]}, {"start": 3, "end": 4, "lines": ["Bye"]}]}`,
			want: []model.ParseWarning{
				{Message: "The field metadata.title must be a string.", Block: model.NoBlock, Lines: []string{`{"title":5}`}, Action: model.Skipped},
				{Message: "The field comments[0].text must be a string.", Block: model.NoBlock, Lines: []string{`{"text":5,"beforeCueIndex":0}`}, Action: model.Skipped},
			},
		},
		{
			name:    "bad file format data",
			content: `{"version": 1, "metadata": [], "formatData": {"csv": {"header": null, "width": 2}}, "cues": [{"start": 1, "end": 2, "lines": ["Hello"]}]}`,
			want: []model.ParseWarning{
				{Message: "The field formatData.csv.roles is missing.", Block: model.NoBlock, Lines: []string{`{"csv":{"header":null,"width":2}}`}, Action: model.Skipped},
			},
		},
		{
			name: "order by block, where no block counts as block 0",
			content: `{"version": 1, "metadata": {"title": 5},
				"formatData": {"x": {"a": {"base64": "!"}}},
				"comments": ["Note"],
				"cues": [{"start": 1, "end": 2, "lines": [7]}, {"start": "1", "end": 2, "lines": []}, {"start": 1, "end": 2, "lines": [],
					"formatData": {"x": {"base64": "!"}}}, {"start": 9, "end": 2, "lines": ["Bad/slash and caf` + "\u00e9" + `"]}]}`,
			want: []model.ParseWarning{
				{Message: "The field formatData.x.a.base64 must be valid base64.", Block: model.NoBlock, Lines: []string{`{"x":{"a":{"base64":"!"}}}`}, Action: model.Skipped},
				{Message: "The field metadata.title must be a string.", Block: model.NoBlock, Lines: []string{`{"title":5}`}, Action: model.Skipped},
				{Message: "The field cues[0].lines[0] must be a string.", Block: 0, Lines: []string{`{"start":1,"end":2,"lines":[7]}`}, Action: model.Skipped},
				{Message: "The field comments[0] must be an object.", Block: model.NoBlock, Lines: []string{`"Note"`}, Action: model.Skipped},
				{Message: "The field cues[1].start must be a number.", Block: 1, Lines: []string{`{"start":"1","end":2,"lines":[]}`}, Action: model.Skipped},
				{
					Message: "The field cues[2].formatData.x.base64 must be valid base64.", Block: 2,
					Lines: []string{`{"start":1,"end":2,"lines":[],"formatData":{"x":{"base64":"!"}}}`}, Action: model.Skipped,
				},
				{
					Message: "The cue ends at 2 s, before it starts at 9 s. The parser swapped the times.", Block: 3,
					Lines: []string{`{"start":9,"end":2,"lines":["Bad/slash and caf` + "\u00e9" + `"]}`}, Action: model.Repaired,
				},
			},
		},
		{
			name:    "bad base64 after good base64",
			content: `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "formatData": {"x": {"a": {"base64": "AAE="}, "b": {"base64": "!"}}}}]}`,
			want: []model.ParseWarning{
				{
					Message: "The field cues[0].formatData.x.b.base64 must be valid base64.", Block: 0,
					Lines: []string{`{"start":1,"end":2,"lines":[],"formatData":{"x":{"a":{"base64":"AAE="},"b":{"base64":"!"}}}}`}, Action: model.Skipped,
				},
			},
		},
		{
			name: "numbers in block lines",
			content: `{"version": 1, "cues": [{"start": 1.50, "end": 2E0, "lines": [], "identifier": 1e400,
				"x": [-0, 12345678901234567890, 1e-7, 1e21, 0.1]}]}`,
			want: []model.ParseWarning{
				{
					Message: "The field cues[0].identifier must be a string or null.", Block: 0,
					Lines: []string{`{"start":1.5,"end":2.0,"lines":[],"identifier":1e400,"x":[0,12345678901234567000.0,1e-7,1e+21,0.1]}`}, Action: model.Skipped,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, warnings, err := read(t, tt.content, true)
			require.NoError(t, err)
			assert.Equal(t, tt.want, warnings)
		})
	}
}
