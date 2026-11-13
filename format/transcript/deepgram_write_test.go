package transcript_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports testARoundTripKeepsTheDataOfTheSource of tests/Formatters/DeepgramFormatterTest.php of subtitle-toolbox-php.
func TestWriteDeepgramRoundTrip(t *testing.T) {
	t.Parallel()

	deepgramFormat.testRoundTrip(t)
}

func setChannel(c *model.Cue, channel int64) {
	data, _ := c.FormatData("deepgram")
	data.Set("channel", channel)
	_ = c.SetFormatData("deepgram", data)
}

// Ports the editing tests of tests/Formatters/DeepgramFormatterTest.php of subtitle-toolbox-php. A test that edited
// word timestamp tags in the lines edits Cue.Words here.
func TestWriteDeepgramEdits(t *testing.T) {
	t.Parallel()

	sauna := `{"word":"sauna","start":1.92,"end":2.38,"confidence":1.0,"speaker":0,"punctuated_word":"sauna"}`
	tests := []struct {
		name   string
		file   string
		opts   transcript.ReadOptions
		index  int
		edit   func(c *model.Cue)
		checks []cloudCheck
		absent [][]any
	}{
		{
			name: "an edited word gets the confidence of a person in the utterance and the channel",
			file: "pool_utterances_diarize.json", opts: cloudWords, index: 0,
			edit: func(c *model.Cue) {
				c.Lines = []string{"Good morning, is the sauna open today?"}
				c.Words[4].Text = "sauna"
			},
			checks: []cloudCheck{
				{[]any{"results", "utterances", 0, "transcript"}, `"Good morning, is the sauna open today?"`},
				{[]any{"results", "utterances", 0, "words", 4}, sauna},
				{[]any{"results", "channels", 0, "alternatives", 0, "words", 4}, sauna},
				{[]any{"results", "utterances", 0, "words", 3, "confidence"}, `0.9087655`},
				{[]any{"results", "utterances", 0, "start"}, `0.23999998`},
			},
		},
		{
			name: "a new speaker changes the utterance and its words",
			file: "pool_utterances_diarize.json", opts: cloudWords, index: 1,
			edit: func(c *model.Cue) { c.Speaker = "2" },
			checks: []cloudCheck{
				{[]any{"results", "utterances", 1, "speaker"}, `2`},
				{[]any{"results", "utterances", 1, "words", 0, "speaker"}, `2`},
				{[]any{"results", "utterances", 1, "words", 3, "speaker"}, `2`},
			},
		},
		{
			name: "an edited sentence stays in its paragraph",
			file: "train_paragraphs_german.json", index: 1,
			edit: func(c *model.Cue) { c.Lines = []string{"Bitte steigen Sie hinten ein."} },
			checks: []cloudCheck{
				{
					[]any{"results", "channels", 0, "alternatives", 0, "paragraphs", "transcript"},
					"\"\\nDer Zug nach Hamburg f\u00e4hrt um zehn Uhr ab. Bitte steigen Sie hinten ein.\\n\\nDer Speisewagen ist heute geschlossen.\"",
				},
				{
					[]any{"results", "channels", 0, "alternatives", 0, "paragraphs", "paragraphs", 0, "sentences", 1},
					`{"text":"Bitte steigen Sie hinten ein.","start":4.34,"end":6.2}`,
				},
				{[]any{"results", "channels", 0, "alternatives", 0, "paragraphs", "paragraphs", 0, "num_words"}, `14`},
				{
					[]any{"results", "channels", 0, "alternatives", 0, "words", 12},
					`{"word":"hinten","start":5.44,"end":5.74,"confidence":1.0,"punctuated_word":"hinten"}`,
				},
				{[]any{"results", "channels", 0, "alternatives", 0, "words", 11, "confidence"}, `0.9087655`},
			},
		},
		{
			name: "a cue on a channel that the file lacks adds the channel",
			file: "museum_words_punctuate.json", index: 0,
			edit: func(c *model.Cue) {
				setChannel(c, 1)
			},
			checks: []cloudCheck{{[]any{"results", "channels", 1, "alternatives", 0, "words", 0, "word"}, `"the"`}},
		},
		{
			name: "a channel number far above the cue count counts as channel 0",
			file: "museum_words_punctuate.json", index: 0,
			edit: func(c *model.Cue) {
				setChannel(c, 1<<40)
			},
			checks: []cloudCheck{{[]any{"results", "channels", 0, "alternatives", 0, "words", 0, "word"}, `"the"`}},
			absent: [][]any{{"results", "channels", 1}},
		},
		{
			name: "a typed end and confidence of a reused word reach the word",
			file: "museum_words_punctuate.json", opts: cloudWords, index: 0,
			edit: func(c *model.Cue) {
				c.Words[0].End = cloudMS(500)
				conf := 0.5
				c.Words[0].Confidence = &conf
			},
			checks: []cloudCheck{
				{[]any{"results", "channels", 0, "alternatives", 0, "words", 0, "end"}, `0.5`},
				{[]any{"results", "channels", 0, "alternatives", 0, "words", 0, "confidence"}, `0.5`},
				{[]any{"results", "channels", 0, "alternatives", 0, "words", 1, "start"}, `0.57999998`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := deepgramFormat.parseFile(t, tt.file, tt.opts)
			if tt.edit != nil {
				cloudEdit(t, s, tt.index, tt.edit)
			}
			doc := deepgramFormat.output(t, s)
			for _, c := range tt.checks {
				assert.Equal(t, c.want, cloudAt(t, doc, c.path...), c.path)
			}
			for _, path := range tt.absent {
				assert.False(t, cloudHasPath(doc, path...), path)
			}
		})
	}
}

// Ports testCuesFromAnotherFormatGiveUtterancesAndWordsForTheWordTimestamps of the same file.
func TestWriteDeepgramFromAnotherFormat(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataLanguage, "de")
	s.AddCue(model.Cue{Start: cloudMS(1500), End: cloudMS(3000), Lines: []string{"Guten", "Morgen."}, Speaker: "1"})
	s.AddCue(model.Cue{
		Start: cloudMS(4000), End: cloudMS(5000), Lines: []string{"Bis bald."},
		Words: []model.Word{{Text: "Bis", Start: cloudMS(4000), End: cloudMS(4500)}, {Text: "bald.", Start: cloudMS(4500), End: cloudMS(5000)}},
	})
	want := `{"results":{"channels":[{"alternatives":[{"transcript":"Guten Morgen. Bis bald.","words":[` +
		`{"word":"bis","start":4.0,"end":4.5,"confidence":1.0,"punctuated_word":"Bis"},` +
		`{"word":"bald","start":4.5,"end":5.0,"confidence":1.0,"punctuated_word":"bald."}]}],"detected_language":"de"}],` +
		`"utterances":[{"start":1.5,"end":3.0,"confidence":1.0,"channel":0,"transcript":"Guten Morgen.","words":[],"speaker":1},` +
		`{"start":4.0,"end":5.0,"confidence":1.0,"channel":0,"transcript":"Bis bald.","words":[` +
		`{"word":"bis","start":4.0,"end":4.5,"confidence":1.0,"punctuated_word":"Bis"},` +
		`{"word":"bald","start":4.5,"end":5.0,"confidence":1.0,"punctuated_word":"bald."}]}]}}`
	assert.Equal(t, want, orderedjson.Block(deepgramFormat.output(t, s)))
}

// A word with an end of 0.65 s and a confidence of 0.93 keeps both values in a Deepgram word.
func TestWriteDeepgramKeepsTheEndAndConfidenceOfAWord(t *testing.T) {
	t.Parallel()

	conf := 0.93
	s := &model.Subtitle{}
	s.AddCue(model.Cue{
		End: cloudMS(1000), Lines: []string{"beach"},
		Words: []model.Word{{Text: "beach", Start: cloudMS(100), End: cloudMS(650), Confidence: &conf}},
	})
	doc := deepgramFormat.output(t, s)
	assert.JSONEq(t, `{"word":"beach","start":0.1,"end":0.65,"confidence":0.93,"punctuated_word":"beach"}`,
		cloudAt(t, doc, "results", "channels", 0, "alternatives", 0, "words", 0))
}
