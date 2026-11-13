package transcript_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var awsFormat = cloudFormat{folder: "aws-transcribe", read: transcript.ReadAWSTranscribe, write: transcript.WriteAWSTranscribe}

const awsHelloWorld = `{"jobName": "lecture-12", "results": {
	"transcripts": [{"transcript": "Hello world."}],
	"items": [
		{"type": "pronunciation", "start_time": "0.04", "end_time": "0.51", "alternatives": [{"confidence": "0.99", "content": "Hello"}]},
		{"type": "pronunciation", "start_time": "0.51", "end_time": "0.98", "alternatives": [{"confidence": "0.98", "content": "world"}]},
		{"type": "punctuation", "alternatives": [{"confidence": "0.0", "content": "."}]}
	]}}`

// awsItem returns a pronunciation item. id is JSON text.
func awsItem(id, start, end, content string) string {
	return `{"id": ` + id + `, "type": "pronunciation", "start_time": "` + start + `", "end_time": "` + end +
		`", "alternatives": [{"confidence": "0.9", "content": "` + content + `"}]}`
}

// Ports the read tests of tests/Parsers/CloudSpeechParsersTest.php of subtitle-toolbox-php.
func TestReadAWSTranscribeCues(t *testing.T) {
	t.Parallel()

	segments := func(list string) string {
		return `{"results": {"items": [` + awsItem("5", "1", "2", "five") + `,` + awsItem(`"7"`, "2", "3", "seven") + `,` +
			awsItem(`"05"`, "3", "4", "zero") + `], "audio_segments": [` + list + `]}}`
	}
	segment := func(items string) string {
		return `{"start_time": "1", "end_time": "4", "transcript": "five seven zero", "items": [` + items + `]}`
	}
	tests := []struct {
		name    string
		content string
		opts    transcript.ReadOptions
		want    []string
	}{
		{"one cue per job without segments", awsHelloWorld, transcript.ReadOptions{}, []string{"0.04-0.98 Hello world."}},
		{"word timestamps", awsHelloWorld, transcript.ReadOptions{WordTimestamps: true}, []string{"0.04-0.98 <00:00:00.040>Hello <00:00:00.510>world."}},
		{
			name: "a leading punctuation item joins the next word",
			content: `{"results": {"transcripts": [], "items": [{"type": "punctuation", "alternatives": [{"content": "\u00bf"}]},` +
				`{"type": "pronunciation", "start_time": "1.0", "end_time": "1.5", "alternatives": [{"content": "Vienes"}]},` +
				`{"type": "punctuation", "alternatives": [{"content": "?"}]}]}}`,
			want: []string{"1-1.5 \u00bfVienes?"},
		},
		{
			name: "the speaker of each item without segments",
			content: `{"results": {"transcripts": [], "items": [` +
				`{"type": "pronunciation", "start_time": "0.1", "end_time": "0.4", "speaker_label": "spk_0", "alternatives": [{"content": "Yes"}]},` +
				`{"type": "pronunciation", "start_time": "0.5", "end_time": "0.9", "speaker_label": "spk_1", "alternatives": [{"content": "No"}]}]}}`,
			opts: transcript.ReadOptions{SpeakerVoices: true},
			want: []string{"0.1-0.4 <v spk_0>Yes", "0.5-0.9 <v spk_1>No"},
		},
		{
			"the integer 5 and the string 5 name one item, 05 names another",
			segments(segment(`"5", 7, "05"`)),
			transcript.ReadOptions{WordTimestamps: true},
			[]string{"1-4 <00:00:01.000>five <00:00:02.000>seven <00:00:03.000>zero"},
		},
		{
			"an id that names no word is ignored and a word named twice in a row counts once",
			segments(segment(`5, "5", 99, 7`)),
			transcript.ReadOptions{WordTimestamps: true},
			[]string{"1-4 <00:00:01.000>five <00:00:02.000>seven zero"},
		},
		{
			"a segment without items has no words", segments(`{"start_time": "1", "end_time": "4", "transcript": "five seven zero"}`),
			transcript.ReadOptions{WordTimestamps: true},
			[]string{"1-4 five seven zero"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, cloudDescribe(awsFormat.parse(t, tt.content, tt.opts)))
		})
	}
}

func TestReadAWSTranscribeData(t *testing.T) {
	t.Parallel()

	s := awsFormat.parse(t, awsHelloWorld, transcript.ReadOptions{})
	file, _ := s.FormatData("aws-transcribe")
	assert.JSONEq(t, `{"jobName":"lecture-12"}`, awsJSONOf(file))
	c, _ := s.Cue(0)
	cue, _ := c.FormatData("aws-transcribe")
	assert.JSONEq(t, `{"items":[{"type":"pronunciation","start_time":"0.04","end_time":"0.51","alternatives":[{"confidence":"0.99","content":"Hello"}]},`+
		`{"type":"pronunciation","start_time":"0.51","end_time":"0.98","alternatives":[{"confidence":"0.98","content":"world"}]},`+
		`{"type":"punctuation","alternatives":[{"confidence":"0.0","content":"."}]}]}`, awsJSONOf(cue))

	s = awsFormat.parseFile(t, "library_speakers_language_id.json", transcript.ReadOptions{})
	file, _ = s.FormatData("aws-transcribe")
	assert.Equal(t, []string{"jobName", "accountId", "status", "results"}, file.Keys())
	results, _ := file.Get("results")
	assert.JSONEq(t, `{"speaker_labels":{"channel_label":"ch_0","speakers":2},"language_code":"en-GB",`+
		`"language_identification":[{"score":"0.9871","code":"en-GB"},{"score":"0.0102","code":"en-AU"},{"score":"0.0027","code":"en-US"}]}`,
		awsJSONOf(results))
	language, _ := s.Metadata(model.MetadataLanguage)
	assert.Equal(t, "en-GB", language)
}

func awsJSONOf(v any) string {
	out, err := jsondata.FromData(v)
	if err != nil {
		return err.Error()
	}
	return orderedjson.Block(out)
}

func TestReadAWSTranscribeErrors(t *testing.T) {
	t.Parallel()

	item := func(rest string) string {
		return `{"results": {"items": [{"type": "pronunciation", ` + rest + `},` + awsItem("1", "1", "2", "there") + `]}}`
	}
	awsFormat.testBroken(t, []cloudBroken{
		{
			"start", item(`"start_time": "soon", "end_time": "1", "alternatives": [{"content": "Hi"}]`),
			"The field results.items[0].start_time must be a time.",
			[]string{"1-2 there"},
			0,
		},
		{
			"end", item(`"start_time": "0", "end_time": -1, "alternatives": [{"content": "Hi"}]`),
			"The field results.items[0].end_time must be a time.",
			[]string{"1-2 there"},
			0,
		},
		{
			"content", item(`"start_time": "0", "end_time": "1", "alternatives": [{"content": 5}]`),
			"The field results.items[0].alternatives[0].content must be a string.",
			[]string{"1-2 there"},
			0,
		},
		{
			"no alternatives", item(`"start_time": "0", "end_time": "1"`),
			"The field results.items[0].alternatives[0].content must be a string.",
			[]string{"1-2 there"},
			0,
		},
		{
			"over the limit", item(`"start_time": "360000000", "end_time": "360000001", "alternatives": [{"content": "Hi"}]`),
			"The field results.items[0].start_time is not below 100000 hours.",
			[]string{"1-2 there"},
			0,
		},
		{
			"segment end",
			`{"results": {"items": [], "audio_segments": [{"start_time": "0", "end_time": "x", "transcript": "a"}, ` +
				`{"start_time": "1", "end_time": "2", "transcript": "b"}]}}`,
			"The field results.audio_segments[0].end_time must be a time.",
			[]string{"1-2 b"},
			0,
		},
		{
			"segment text",
			`{"results": {"items": [], "audio_segments": [{"start_time": "0", "end_time": "1"}, {"start_time": "1", "end_time": "2", "transcript": "b"}]}}`,
			"The field results.audio_segments[0].transcript must be a string.",
			[]string{"1-2 b"},
			0,
		},
		{
			"segment that is no object", `{"results": {"items": [], "audio_segments": [5, {"start_time": "1", "end_time": "2", "transcript": "b"}]}}`,
			"The field results.audio_segments[0].start_time must be a time.",
			[]string{"1-2 b"},
			0,
		},
	})

	for _, content := range []string{`{"results": {}}`, `{"results": []}`, `{"results": {"items": {"0": {}}}}`, `{}`, `{"results": {"items": null}}`} {
		_, _, err := transcript.ReadAWSTranscribe(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, transcript.ReadOptions{})
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr, content)
		assert.Equal(t, `The JSON has no "results.items" list.`, perr.Msg)
	}
	for content, msg := range map[string]string{"[1]": "The JSON root must be an object.", "{": "The content is not valid JSON: "} {
		_, _, err := transcript.ReadAWSTranscribe(strings.NewReader(content), model.CommonReadOptions{}, transcript.ReadOptions{})
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr)
		assert.True(t, strings.HasPrefix(perr.Msg, msg), perr.Msg)
	}
}
