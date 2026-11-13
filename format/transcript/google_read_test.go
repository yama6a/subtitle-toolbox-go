package transcript_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var googleFormat = cloudFormat{folder: "google-speech", read: transcript.ReadGoogleSpeech, write: transcript.WriteGoogleSpeech}

// Ports the read tests of tests/Parsers/CloudSpeechParsersTest.php of subtitle-toolbox-php.
func TestReadGoogleSpeechCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		opts    transcript.ReadOptions
		want    []string
	}{
		{
			"V1 diarization reads the words of the last result",
			`{"results": [` +
				`{"alternatives": [{"transcript": "hi there", "words": [{"startTime": "0s", "endTime": "0.300s", "word": "hi"}, ` +
				`{"startTime": "0.300s", "endTime": "0.600s", "word": "there"}]}]},` +
				`{"alternatives": [{"words": [{"startTime": "0s", "endTime": "0.300s", "word": "hi", "speakerTag": 1}, ` +
				`{"startTime": "0.300s", "endTime": "0.600s", "word": "there", "speakerTag": 2}]}]}]}`,
			transcript.ReadOptions{SpeakerVoices: true},
			[]string{"0-0.3 <v 1>hi", "0.3-0.6 <v 2>there"},
		},
		{
			"a result without words starts where the result before it ended",
			`{"results": [{"alternatives": [{"transcript": "first part"}], "resultEndTime": "4.500s"},` +
				`{"resultEndTime": "5s"},` +
				`{"alternatives": [{"transcript": " second part"}], "resultEndTime": {"seconds": "9", "nanos": 250000000}}]}`,
			transcript.ReadOptions{},
			[]string{"0-4.5 first part", "5-9.25 second part"},
		},
		{
			"V2 names and an operation",
			`{"name": "op", "done": true, "response": {"results": [{"alternatives": [{"transcript": "hi", "words": [` +
				`{"startOffset": "1s", "endOffset": "2s", "word": "hi"}]}], "resultEndOffset": "2s", "languageCode": "en-us"}]}}`,
			transcript.ReadOptions{WordTimestamps: true},
			[]string{"1-2 <00:00:01.000>hi"},
		},
		{
			"a speaker tag of 0 is no diarization",
			`{"results": [{"alternatives": [{"transcript": "hi", "words": [{"startTime": "0s", "endTime": "1s", "word": "hi", "speakerTag": 0}]}]}]}`,
			transcript.ReadOptions{SpeakerVoices: true},
			[]string{"0-1 hi"},
		},
		{
			"a result with an empty alternatives list",
			`{"results": [{"alternatives": [], "resultEndTime": "2s"}, {"alternatives": [{"transcript": "x"}], "resultEndTime": "3s"}]}`,
			transcript.ReadOptions{},
			[]string{"2-3 x"},
		},
		{"no results gives no cues", `{"results": []}`, transcript.ReadOptions{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, describe(googleFormat.parse(t, tt.content, tt.opts)))
		})
	}
}

func TestReadGoogleSpeechData(t *testing.T) {
	t.Parallel()

	s := googleFormat.parseFile(t, "bus_v1_long_running_operation.json", transcript.ReadOptions{})
	file, _ := s.FormatData("google-speech")
	assert.Equal(t, []string{"name", "metadata", "done", "@type", "totalBilledTime", "requestId"}, file.Keys())
	language, _ := s.Metadata(model.MetadataLanguage)
	assert.Equal(t, "en-us", language)
	c, _ := s.Cue(0)
	data, _ := c.FormatData("google-speech")
	assert.Equal(t, []string{"resultEndTime", "languageCode", "transcript", "confidence", "words"}, data.Keys())

	s = googleFormat.parseFile(t, "restaurant_v2_diarization.json", transcript.ReadOptions{})
	file, _ = s.FormatData("google-speech")
	assert.Equal(t, []string{"results", "metadata"}, file.Keys())
	results, _ := file.Get("results")
	alt, _ := results.([]any)[0].(*model.Data).Get("alternatives")
	assert.Equal(t, []string{"confidence"}, alt.([]any)[0].(*model.Data).Keys())
}

func TestReadGoogleSpeechErrors(t *testing.T) {
	t.Parallel()

	words := func(list string) string {
		return `{"results": [{"alternatives": [{"transcript": "hi there", "words": [` + list + `]}]}]}`
	}
	prefix := "The field results[0].alternatives[0].words[0]"
	googleFormat.testBroken(t, []cloudBroken{
		{
			"start", words(`{"word": "hi", "startTime": "soon", "endTime": "1s"}, {"word": "there", "startTime": "1s", "endTime": "2s"}`),
			prefix + ".startTime must be a time.",
			[]string{"1-2 hi there"},
			0,
		},
		{
			"V2 end", words(`{"word": "hi", "startOffset": "0s", "endOffset": "-1s"}, {"word": "there", "startTime": "1s", "endTime": "2s"}`),
			prefix + ".endTime must be a time.",
			[]string{"1-2 hi there"},
			0,
		},
		{
			"word", words(`{"word": 5, "startTime": "0s", "endTime": "1s"}, {"word": "there", "startTime": "1s", "endTime": "2s"}`),
			prefix + ".word must be a string.",
			[]string{"1-2 hi there"},
			0,
		},
		{
			"over the limit", words(`{"word": "x", "startTime": "999999999999s", "endTime": "1s"}, {"word": "there", "startTime": "1s", "endTime": "2s"}`),
			prefix + ".startTime is not below 100000 hours.",
			[]string{"1-2 hi there"},
			0,
		},
		{
			"result end", `{"results": [{"alternatives": [{"transcript": "a"}], "resultEndTime": "x"}, {"alternatives": [{"transcript": "b"}], "resultEndTime": "2s"}]}`,
			"The field results[0].resultEndTime must be a time.",
			[]string{"0-2 b"},
			0,
		},
		{
			"transcript", `{"results": [{"alternatives": [{"words": []}], "resultEndTime": "1s"}, {"alternatives": [{"transcript": "b"}], "resultEndTime": "2s"}]}`,
			"The field results[0].alternatives[0].transcript must be a string.",
			[]string{"0-2 b"},
			0,
		},
		{
			"alternatives", `{"results": [{"alternatives": [5], "resultEndTime": "1s"}, {"alternatives": [{"transcript": "b"}], "resultEndTime": "2s"}]}`,
			"The field results[0].alternatives must be a list of objects.",
			[]string{"0-2 b"},
			0,
		},
	})

	for _, content := range []string{`{}`, `{"results": {"0": {}}}`, `{"response": {"results": null}}`, `{"response": []}`} {
		_, _, err := transcript.ReadGoogleSpeech(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, transcript.ReadOptions{})
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr, content)
		assert.Equal(t, `The JSON has no "results" list.`, perr.Msg)
	}
}

// TestReadGoogleSpeechDurations reads a result end in each form of a protobuf Duration.
func TestReadGoogleSpeechDurations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, end string
		want      string
	}{
		{"whole seconds", `"6s"`, "0-6 x"},
		{"decimals", `"1.300s"`, "0-1.3 x"},
		{"a negative string", `"-1s"`, ""},
		{"seconds and nanos as strings", `{"seconds": "3", "nanos": 500000000}`, "0-3.5 x"},
		{"nanos only", `{"nanos": 1}`, "0-0 x"},
		{"a plain number", `1.5`, "0-1.5 x"},
		{"a numeric string", `"1.5"`, "0-1.5 x"},
		{"an empty object", `{}`, ""},
		{"an object with a word", `{"seconds": "x"}`, ""},
		{"an exponent is no duration", `"1e3s"`, ""},
		{"a huge value", `"99999999999999999999s"`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := `{"results": [{"alternatives": [{"transcript": "x"}], "resultEndTime": ` + tt.end + `}]}`
			s, _, err := transcript.ReadGoogleSpeech(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, transcript.ReadOptions{})
			require.NoError(t, err)
			if tt.want == "" {
				assert.Equal(t, 0, s.Len())
				return
			}
			assert.Equal(t, []string{tt.want}, describe(s))
		})
	}
}
