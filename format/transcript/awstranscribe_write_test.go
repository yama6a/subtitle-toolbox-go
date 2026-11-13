package transcript_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports testARoundTripKeepsTheDataOfTheSource of tests/Formatters/AwsTranscribeFormatterTest.php of subtitle-toolbox-php.
func TestWriteAWSTranscribeRoundTrip(t *testing.T) {
	t.Parallel()

	awsFormat.testRoundTrip(t)
}

// Ports the editing tests of tests/Formatters/AwsTranscribeFormatterTest.php of subtitle-toolbox-php. A test that edited
// word timestamp tags in the lines edits Cue.Words here.
func TestWriteAWSTranscribeEdits(t *testing.T) {
	t.Parallel()

	const library = "library_speakers_language_id.json"
	tests := []struct {
		name  string
		file  string
		opts  transcript.ReadOptions
		index int
		edit  func(c *model.Cue)
		// checks and absent are expectations on the written document.
		checks []cloudCheck
		absent [][]any
	}{
		{
			name: "an edited word gets the confidence of a person and keeps its punctuation",
			file: library, opts: cloudWords, index: 0,
			edit: func(c *model.Cue) {
				c.Lines = []string{"The library opens at ten."}
				c.Words[4].Text = "ten"
			},
			checks: []cloudCheck{
				{[]any{"results", "audio_segments", 0, "transcript"}, `"The library opens at ten."`},
				{[]any{"results", "items", 4}, `{"id":4,"start_time":"2.0","speaker_label":"spk_0","end_time":"2.3","alternatives":[{"confidence":"1.0","content":"ten"}],"type":"pronunciation"}`},
				{[]any{"results", "items", 5, "alternatives", 0}, `{"confidence":"0.0","content":"."}`},
				{[]any{"results", "items", 2, "alternatives", 0, "confidence"}, `"0.95"`},
			},
		},
		{
			name: "a new word gets new items and the following ids move",
			file: library, opts: cloudWords, index: 0,
			edit: func(c *model.Cue) {
				c.Lines = []string{"The library opens at nine sharp."}
				c.Words[4] = model.Word{Text: "nine", Start: cloudMS(2000), End: cloudMS(2200)}
				c.Words = append(c.Words, model.Word{Text: "sharp.", Start: cloudMS(2200), End: cloudMS(2300)})
			},
			checks: []cloudCheck{
				{[]any{"results", "items", 4, "alternatives", 0, "content"}, `"nine"`},
				{[]any{"results", "items", 4, "end_time"}, `"2.2"`},
				{[]any{"results", "items", 5}, `{"id":5,"start_time":"2.2","speaker_label":"spk_0","end_time":"2.3","alternatives":[{"confidence":"1.0","content":"sharp"}],"type":"pronunciation"}`},
				{[]any{"results", "items", 6, "alternatives", 0}, `{"confidence":"0.0","content":"."}`},
				{[]any{"results", "audio_segments", 0, "items"}, `[0,1,2,3,4,5,6]`},
				{[]any{"results", "audio_segments", 1, "items", 0}, `7`},
				{[]any{"results", "items", 22, "id"}, `22`},
			},
		},
		{
			name: "a new speaker changes the items and the speaker segments",
			file: library, opts: cloudWords, index: 1,
			edit: func(c *model.Cue) { c.Speaker = "spk_0" },
			checks: []cloudCheck{
				{[]any{"results", "audio_segments", 1, "speaker_label"}, `"spk_0"`},
				{[]any{"results", "speaker_labels", "segments", 0, "start_time"}, `"0.52"`},
				{[]any{"results", "speaker_labels", "segments", 0, "end_time"}, `"8.6"`},
				{[]any{"results", "speaker_labels", "speakers"}, `1`},
				{[]any{"results", "speaker_labels", "channel_label"}, `"ch_0"`},
			},
			absent: [][]any{{"results", "speaker_labels", "segments", 1}},
		},
		{
			name: "a word changed without word times keeps the times of its source word",
			file: library, index: 2,
			edit: func(c *model.Cue) { c.Lines = []string{"Yes, use the box by the gate."} },
			checks: []cloudCheck{
				{[]any{"results", "items", 20}, `{"id":20,"start_time":"8.3","end_time":"8.6","alternatives":[{"confidence":"1.0","content":"gate"}],"type":"pronunciation"}`},
				{[]any{"results", "items", 21, "alternatives", 0}, `{"confidence":"0.0","content":"."}`},
				{[]any{"results", "items", 19, "alternatives", 0, "confidence"}, `"0.91"`},
			},
		},
		{
			name: "a text with other words and no word times has no items and keeps its audio segment",
			file: library, index: 2,
			edit: func(c *model.Cue) { c.Lines = []string{"Yes, use the box by the front door."} },
			checks: []cloudCheck{
				{[]any{"results", "audio_segments", 2, "items"}, `[]`},
				{[]any{"results", "audio_segments", 2, "transcript"}, `"Yes, use the box by the front door."`},
				{[]any{"results", "items", 12, "id"}, `12`},
			},
			absent: [][]any{{"results", "items", 13}},
		},
		{
			name: "a cue without words in an items-only file adds audio segments",
			file: "weather_items_only.json", index: 0,
			edit: func(c *model.Cue) { c.Lines = []string{"Rain is expected in the south."} },
			checks: []cloudCheck{
				{[]any{"results", "audio_segments", 0, "items"}, `[]`},
				{[]any{"results", "audio_segments", 0, "start_time"}, `"0.31"`},
				{[]any{"results", "audio_segments", 0, "end_time"}, `"3.43"`},
				{[]any{"results", "audio_segments", 4, "id"}, `4`},
			},
			absent: [][]any{{"results", "audio_segments", 5}},
		},
		{
			name: "a moved cue writes its new times as strings",
			file: library, index: 1,
			edit: func(c *model.Cue) { c.Start, c.End = cloudMS(3000), cloudMS(5250) },
			checks: []cloudCheck{
				{[]any{"results", "audio_segments", 1, "start_time"}, `"3.0"`},
				{[]any{"results", "audio_segments", 1, "end_time"}, `"5.25"`},
			},
		},
		{
			name: "a typed end and confidence of a reused word reach the item",
			file: library, opts: cloudWords, index: 0,
			edit: func(c *model.Cue) {
				c.Words[1].End = cloudMS(1300)
				confidence := 0.9989
				c.Words[1].Confidence = &confidence
			},
			checks: []cloudCheck{
				{[]any{"results", "items", 1, "end_time"}, `"1.3"`},
				{[]any{"results", "items", 1, "alternatives", 0, "confidence"}, `"0.9989"`},
				{[]any{"results", "items", 0, "alternatives", 0, "confidence"}, `"0.91"`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := awsFormat.parseFile(t, tt.file, tt.opts)
			cloudEdit(t, s, tt.index, tt.edit)
			doc := awsFormat.output(t, s)
			for _, c := range tt.checks {
				assert.Equal(t, c.want, cloudAt(t, doc, c.path...), c.path)
			}
			for _, path := range tt.absent {
				assert.False(t, cloudHasPath(doc, path...), path)
			}
		})
	}
}

func TestWriteAWSTranscribeAnItemsOnlyFileReadsBack(t *testing.T) {
	t.Parallel()

	s := awsFormat.parseFile(t, "weather_items_only.json", transcript.ReadOptions{})
	cloudEdit(t, s, 0, func(c *model.Cue) { c.Lines = []string{"Rain is expected in the south."} })
	var out []byte
	{
		doc := awsFormat.output(t, s)
		out = []byte(orderedjson.Block(doc))
	}
	back := awsFormat.parse(t, string(out), transcript.ReadOptions{})
	assert.Equal(t, 5, back.Len())
}

// Ports testCuesFromAnotherFormatGiveAudioSegmentsAndItemsForTheWordTimestamps of the same file.
func TestWriteAWSTranscribeFromAnotherFormat(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataLanguage, "en-GB")
	s.AddCue(model.Cue{Start: cloudMS(1500), End: cloudMS(3000), Lines: []string{"Good", "morning."}, Speaker: "Anna"})
	s.AddCue(model.Cue{
		Start: cloudMS(4000), End: cloudMS(5000), Lines: []string{"See you."}, Speaker: "Ben",
		Words: []model.Word{{Text: "See", Start: cloudMS(4000), End: cloudMS(4500)}, {Text: "you.", Start: cloudMS(4500), End: cloudMS(5000)}},
	})
	want := `{"results":{"transcripts":[{"transcript":"Good morning. See you."}],` +
		`"speaker_labels":{"speakers":1,"segments":[{"start_time":"4.0","speaker_label":"Ben","end_time":"5.0","items":[` +
		`{"start_time":"4.0","speaker_label":"Ben","end_time":"4.5"},{"start_time":"4.5","speaker_label":"Ben","end_time":"5.0"}]}]},` +
		`"items":[{"id":0,"start_time":"4.0","speaker_label":"Ben","end_time":"4.5","alternatives":[{"confidence":"1.0","content":"See"}],"type":"pronunciation"},` +
		`{"id":1,"start_time":"4.5","speaker_label":"Ben","end_time":"5.0","alternatives":[{"confidence":"1.0","content":"you"}],"type":"pronunciation"},` +
		`{"id":2,"speaker_label":"Ben","alternatives":[{"confidence":"0.0","content":"."}],"type":"punctuation"}],` +
		`"audio_segments":[{"id":0,"transcript":"Good morning.","start_time":"1.5","end_time":"3.0","speaker_label":"Anna","items":[]},` +
		`{"id":1,"transcript":"See you.","start_time":"4.0","end_time":"5.0","speaker_label":"Ben","items":[0,1,2]}],` +
		`"language_code":"en-GB"}}`
	assert.Equal(t, want, orderedjson.Block(awsFormat.output(t, s)))
}

func TestWriteAWSTranscribeErrors(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{End: cloudMS(1000), Lines: []string{"text"}, Speaker: "bad \xff"})
	err := transcript.WriteAWSTranscribe(io.Discard, s, model.CommonWriteOptions{}, transcript.WriteOptions{})
	require.ErrorIs(t, err, model.ErrUnwritable)
	require.ErrorIs(t, err, model.ErrInvalidUTF8)
}
