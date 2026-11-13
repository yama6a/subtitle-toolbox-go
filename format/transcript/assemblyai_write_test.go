package transcript_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports testARoundTripKeepsTheDataOfTheSource of tests/Formatters/AssemblyAiFormatterTest.php of subtitle-toolbox-php.
func TestWriteAssemblyAIRoundTrip(t *testing.T) {
	t.Parallel()

	assemblyAIFormat.testRoundTrip(t)
}

// Ports the editing tests of tests/Formatters/AssemblyAiFormatterTest.php of subtitle-toolbox-php. A test that edited
// word timestamp tags in the lines edits Cue.Words here.
func TestWriteAssemblyAIEdits(t *testing.T) {
	t.Parallel()

	walk := `{"text":"walk","start":2580,"end":3040,"confidence":1.0,"speaker":"A"}`
	tests := []struct {
		name   string
		file   string
		opts   transcript.ReadOptions
		index  int
		edit   func(c *model.Cue)
		checks []check
	}{
		{
			name: "an edited word gets the confidence of a person in the utterance and the words",
			file: "hike_utterances_speakers.json", opts: withWords, index: 0,
			edit: func(c *model.Cue) {
				c.Lines = []string{"Where should we meet for the walk tomorrow?"}
				c.Words[6].Text = "walk"
			},
			checks: []check{
				{[]any{"utterances", 0, "text"}, `"Where should we meet for the walk tomorrow?"`},
				{[]any{"utterances", 0, "words", 6}, walk},
				{[]any{"words", 6}, walk},
				{[]any{"words", 5, "confidence"}, `0.90655`},
			},
		},
		{
			name: "a new speaker changes the utterance and its words",
			file: "hike_utterances_speakers.json", opts: withWords, index: 1,
			edit: func(c *model.Cue) { c.Speaker = "C" },
			checks: []check{
				{[]any{"utterances", 1, "speaker"}, `"C"`},
				{[]any{"utterances", 1, "words", 0, "speaker"}, `"C"`},
				{[]any{"utterances", 1, "words", 5, "speaker"}, `"C"`},
			},
		},
		{
			name: "a moved cue without word times loses its words in a words-only file",
			file: "market_words_french.json", index: 1,
			edit: func(c *model.Cue) { c.Start = ms(3000) },
			checks: []check{
				{[]any{"utterances", 1, "start"}, `3000`},
				{[]any{"utterances", 1, "words"}, `[]`},
				{[]any{"utterances", 0, "start"}, `140`},
				{[]any{"utterances", 0, "end"}, `2340`},
			},
		},
		{
			name: "a typed end and confidence of a reused word reach the word",
			file: "hike_utterances_speakers.json", opts: withWords, index: 0,
			edit: func(c *model.Cue) {
				c.Words[0].End = ms(500)
				conf := 0.5
				c.Words[0].Confidence = &conf
			},
			checks: []check{
				{[]any{"words", 0}, `{"text":"Where","start":260,"end":500,"confidence":0.5,"speaker":"A"}`},
				{[]any{"words", 1, "confidence"}, `0.91655`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := assemblyAIFormat.parseFile(t, tt.file, tt.opts)
			editCue(t, s, tt.index, tt.edit)
			doc := assemblyAIFormat.output(t, s)
			for _, c := range tt.checks {
				assert.JSONEq(t, c.want, compact(t, doc, c.path...), c.path)
			}
			utterances := compact(t, doc, "utterances")
			assert.NotEqual(t, "null", utterances)
		})
	}
}

func TestWriteAssemblyAIAMovedCueReadsBack(t *testing.T) {
	t.Parallel()

	s := assemblyAIFormat.parseFile(t, "market_words_french.json", transcript.ReadOptions{})
	editCue(t, s, 1, func(c *model.Cue) { c.Start = ms(3000) })
	back := assemblyAIFormat.parse(t, orderedjson.Block(assemblyAIFormat.output(t, s)), transcript.ReadOptions{})
	assert.Equal(t, 4, back.Len())
}

// Ports testCuesFromAnotherFormatGiveWordsAndUtterancesNullWhenEveryCueHasWords and
// testCuesFromAnotherFormatWithSpeakersGiveUtterances of the same file.
func TestWriteAssemblyAIFromAnotherFormat(t *testing.T) {
	t.Parallel()

	words := []model.Word{{Text: "See", Start: ms(4000), End: ms(4500)}, {Text: "you.", Start: ms(4500), End: ms(5000)}}
	tests := []struct {
		name string
		cue  model.Cue
		lang bool
		want string
	}{
		{
			"every cue has words",
			model.Cue{Start: ms(4000), End: ms(5000), Lines: []string{"See you."}, Words: words},
			true,
			`{"language_code":"en_us","text":"See you.","words":[{"text":"See","start":4000,"end":4500,"confidence":1.0,"speaker":null},` +
				`{"text":"you.","start":4500,"end":5000,"confidence":1.0,"speaker":null}],"utterances":null}`,
		},
		{
			"a cue with a speaker",
			model.Cue{Start: ms(1500), End: ms(3000), Lines: []string{"Good", "morning."}, Speaker: "Anna"},
			false,
			`{"text":"Good morning.","words":[],"utterances":[{"confidence":1.0,"end":3000,"speaker":"Anna","start":1500,"text":"Good morning.","words":[]}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &model.Subtitle{}
			if tt.lang {
				s.SetMetadata(model.MetadataLanguage, "en-US")
			}
			s.AddCue(tt.cue)
			assert.Equal(t, tt.want, orderedjson.Block(assemblyAIFormat.output(t, s)))
		})
	}
}
