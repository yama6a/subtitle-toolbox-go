package transcript_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var deepgramFormat = cloudFormat{folder: "deepgram", read: transcript.ReadDeepgram, write: transcript.WriteDeepgram}

// Ports the read tests of tests/Parsers/CloudSpeechParsersTest.php of subtitle-toolbox-php.
func TestReadDeepgramCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		opts    transcript.ReadOptions
		want    []string
	}{
		{
			"every channel in time order",
			`{"results": {"channels": [` +
				`{"alternatives": [{"words": [{"word": "later", "start": 2.0, "end": 2.5}]}]},` +
				`{"alternatives": [{"words": [{"word": "first", "start": 0.5, "end": 1.0, "punctuated_word": "First."}]}]}]}}`,
			transcript.ReadOptions{},
			[]string{"0.5-1 First.", "2-2.5 later"},
		},
		{
			"paragraphs use the speaker of the paragraph",
			`{"results": {"channels": [{"alternatives": [{"words": [], "paragraphs": {"paragraphs": [` +
				`{"speaker": 1, "sentences": [{"text": "Hello there.", "start": 0.2, "end": 1.1}]}]}}]}]}}`,
			transcript.ReadOptions{SpeakerVoices: true},
			[]string{"0.2-1.1 <v 1>Hello there."},
		},
		{
			"a null punctuated word falls back to the word",
			`{"results": {"channels": [{"alternatives": [{"words": [{"word": "hi", "punctuated_word": null, "start": 0, "end": 1}]}]}]}}`,
			transcript.ReadOptions{},
			[]string{"0-1 hi"},
		},
		{
			"utterances win over the words and keep word times",
			`{"results": {"channels": [{"alternatives": [{"words": [{"word": "x", "start": 5, "end": 6}]}]}], "utterances": [` +
				`{"start": 0, "end": 2, "transcript": "Hi there.", "speaker": 3, "words": [` +
				`{"word": "hi", "start": 0, "end": 1, "punctuated_word": "Hi"}, {"word": "there", "start": 1, "end": 2, "punctuated_word": "there."}]}]}}`,
			transcript.ReadOptions{WordTimestamps: true, SpeakerVoices: true},
			[]string{"0-2 <v 3><00:00:00.000>Hi <00:00:01.000>there."},
		},
		{
			"an empty utterance list falls back to the channels",
			`{"results": {"utterances": [], "channels": [{"alternatives": [{"words": [{"word": "a", "start": 0, "end": 1}]}]}]}}`,
			transcript.ReadOptions{},
			[]string{"0-1 a"},
		},
		{
			"a channel without an alternative gives no cue",
			`{"results": {"channels": [{"alternatives": []}, 5, {"alternatives": [{"words": [{"word": "a", "start": 0, "end": 1}]}]}]}}`,
			transcript.ReadOptions{},
			[]string{"0-1 a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, cloudDescribe(deepgramFormat.parse(t, tt.content, tt.opts)))
		})
	}
}

func TestReadDeepgramData(t *testing.T) {
	t.Parallel()

	s := deepgramFormat.parseFile(t, "train_paragraphs_german.json", transcript.ReadOptions{})
	language, _ := s.Metadata(model.MetadataLanguage)
	assert.Equal(t, "de", language)
	file, _ := s.FormatData("deepgram")
	results, _ := file.Get("results")
	channels, _ := results.(*model.Data).Get("channels")
	channel := channels.([]any)[0]
	assert.Equal(t, []string{"alternatives", "detected_language", "language_confidence"}, channel.(*model.Data).Keys())
	alt, _ := channel.(*model.Data).Get("alternatives")
	assert.Equal(t, []string{"confidence"}, alt.([]any)[0].(*model.Data).Keys())

	c, _ := s.Cue(1)
	data, _ := c.FormatData("deepgram")
	assert.Equal(t, []string{"text", "start", "end", "channel", "paragraph", "words"}, data.Keys())
	words, _ := data.Get("words")
	assert.Len(t, words, 5)
}

func TestReadDeepgramParagraphErrors(t *testing.T) {
	t.Parallel()

	content := `{"results": {"channels": [{"alternatives": [{"words": [], "paragraphs": {"paragraphs": [` +
		`{"sentences": [{"text": "Hi.", "start": 0}, {"text": "Bye.", "start": 1, "end": 2}]},` +
		`{"sentences": [{"text": "Hi again.", "start": 3}]}]}}]}]}}`
	prefix := "The field results.channels[0].alternatives[0].paragraphs.paragraphs"
	s, warnings, err := transcript.ReadDeepgram(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, transcript.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"1-2 Bye."}, cloudDescribe(s))
	assert.Equal(t, []string{
		prefix + "[0].sentences[0].end must be a time.|0|skipped", prefix + "[1].sentences[0].end must be a time.|0|skipped",
	}, cloudWarnings(warnings))
}

func TestReadDeepgramErrors(t *testing.T) {
	t.Parallel()

	words := func(list string) string {
		return `{"results": {"channels": [{"alternatives": [{"words": [` + list + `]}]}]}}`
	}
	prefix := "The field results.channels[0].alternatives[0].words[0]"
	deepgramFormat.testBroken(t, []cloudBroken{
		{
			"end", words(`{"word": "hi", "start": 0}, {"word": "there", "start": 1, "end": 2}`),
			prefix + ".end must be a time.",
			[]string{"1-2 there"},
			0,
		},
		{
			"start", words(`{"word": "hi", "start": "x", "end": 1}, {"word": "there", "start": 1, "end": 2}`),
			prefix + ".start must be a time.",
			[]string{"1-2 there"},
			0,
		},
		{
			"text", words(`{"word": 5, "start": 0, "end": 1}, {"word": "there", "start": 1, "end": 2}`),
			prefix + ".word must be a string.",
			[]string{"1-2 there"},
			0,
		},
		{
			"punctuated text", words(`{"word": "x", "punctuated_word": 5, "start": 0, "end": 1}, {"word": "there", "start": 1, "end": 2}`),
			prefix + ".punctuated_word must be a string.",
			[]string{"1-2 there"},
			0,
		},
		{
			"utterance",
			`{"results": {"channels": [], "utterances": [{"start": 0, "transcript": "Hi"}, {"start": 1, "end": 2, "transcript": "Bye"}]}}`,
			"The field results.utterances[0].end must be a time.",
			[]string{"1-2 Bye"},
			0,
		},
	})

	for _, content := range []string{`{}`, `{"results": []}`, `{"results": {"channels": {"0": {}}}}`, `{"results": {"channels": null}}`} {
		_, _, err := transcript.ReadDeepgram(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, transcript.ReadOptions{})
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr, content)
		assert.Equal(t, `The JSON has no "results.channels" list.`, perr.Msg)
	}
}
