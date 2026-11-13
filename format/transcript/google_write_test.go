package transcript_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports testARoundTripKeepsTheDataOfTheSource of tests/Formatters/GoogleSpeechFormatterTest.php of subtitle-toolbox-php.
func TestWriteGoogleSpeechRoundTrip(t *testing.T) {
	t.Parallel()

	googleFormat.testRoundTrip(t)
}

// Ports the editing tests of tests/Formatters/GoogleSpeechFormatterTest.php of subtitle-toolbox-php. A test that edited
// word timestamp tags in the lines edits Cue.Words here.
func TestWriteGoogleSpeechEdits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		file   string
		opts   transcript.ReadOptions
		index  int
		edit   func(c *model.Cue)
		checks []check
	}{
		{
			name: "an edited word keeps the V1 fields and the operation",
			file: "bus_v1_long_running_operation.json", opts: withWords, index: 1,
			edit: func(c *model.Cue) {
				c.Lines = []string{"the first one is at six thirty in the morning"}
				c.Words[5].Text = "six"
			},
			checks: []check{
				{[]any{"response", "results", 1, "alternatives", 0, "transcript"}, `" the first one is at six thirty in the morning"`},
				{[]any{"response", "results", 1, "alternatives", 0, "words", 5}, `{"startTime":"6.840s","endTime":"7.300s","word":"six"}`},
				{[]any{"response", "results", 1, "resultEndTime"}, `"9.170s"`},
				{[]any{"response", "results", 1, "languageCode"}, `"en-us"`},
				{[]any{"done"}, `true`},
				{[]any{"name"}, `"7612202767953098924"`},
			},
		},
		{
			name: "a moved cue gets a new result end and loses its words without word times",
			file: "bus_v1_long_running_operation.json", index: 0,
			edit: func(c *model.Cue) { c.End = ms(4500) },
			checks: []check{
				{[]any{"response", "results", 0, "resultEndTime"}, `"4.500s"`},
				{[]any{"response", "results", 0, "alternatives", 0, "words"}, `[]`},
			},
		},
		{
			name: "a typed end and confidence of a reused word reach the word",
			file: "bus_v1_long_running_operation.json", opts: withWords, index: 0,
			edit: func(c *model.Cue) {
				c.Words[0].End = ms(700)
				conf := 0.5
				c.Words[0].Confidence = &conf
			},
			checks: []check{
				{[]any{"response", "results", 0, "alternatives", 0, "words", 0, "endTime"}, `"0.700s"`},
				{[]any{"response", "results", 0, "alternatives", 0, "words", 1, "endTime"}, `"1.080s"`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := googleFormat.parseFile(t, tt.file, tt.opts)
			editCue(t, s, tt.index, tt.edit)
			doc := googleFormat.output(t, s)
			for _, c := range tt.checks {
				assert.Equal(t, c.want, compact(t, doc, c.path...), c.path)
			}
		})
	}
}

func TestWriteGoogleSpeechDiarized(t *testing.T) {
	t.Parallel()

	t.Run("a new speaker changes the words of the one result", func(t *testing.T) {
		t.Parallel()

		s := googleFormat.parseFile(t, "restaurant_v2_diarization.json", withWords)
		editCue(t, s, 1, func(c *model.Cue) { c.Speaker = "3" })
		doc := googleFormat.output(t, s)
		results, _ := doc.Get("results")
		require.Len(t, results.Items(), 1)
		var runs []string
		for _, w := range withWordsOf(results.Items()[0]) {
			label := compact(t, w, "speakerLabel")
			if len(runs) == 0 || runs[len(runs)-1] != label {
				runs = append(runs, label)
			}
		}
		assert.Equal(t, []string{`"1"`, `"3"`, `"1"`, `"2"`}, runs)
	})

	t.Run("a cue without words fails", func(t *testing.T) {
		t.Parallel()

		s := googleFormat.parseFile(t, "restaurant_v2_diarization.json", transcript.ReadOptions{})
		editCue(t, s, 2, func(c *model.Cue) { c.Lines = []string{"Around seven o'clock please"} })
		err := transcript.WriteGoogleSpeech(&bytes.Buffer{}, s, model.CommonWriteOptions{}, transcript.WriteOptions{})
		var uerr *model.UnwritableError
		require.ErrorAs(t, err, &uerr)
		assert.Equal(t, 2, uerr.CueIndex)
		assert.Contains(t, uerr.Reason, "Cue #2 at 8.24 s has no words.")
		assert.Contains(t, uerr.Reason, "A Google Speech-to-Text result with speaker diarization holds cues only through their words.")
	})
}

func withWordsOf(result orderedjson.Value) []orderedjson.Value {
	alt, _ := result.Get("alternatives")
	words, _ := alt.Items()[0].Get("words")
	return words.Items()
}

// Ports testAnOverlappingCueEndsItsResultWithTheResultBeforeIt and testCuesFromAnotherFormatGiveV1Results of the same file.
func TestWriteGoogleSpeechFromAnotherFormat(t *testing.T) {
	t.Parallel()

	t.Run("an overlapping cue ends its result with the result before it", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		s.AddCue(model.Cue{Start: ms(1000), End: ms(6000), Lines: []string{"Long cue"}})
		s.AddCue(model.Cue{Start: ms(2000), End: ms(4000), Lines: []string{"Short cue"}})
		doc := googleFormat.output(t, s)
		assert.Equal(t, `"6s"`, compact(t, doc, "results", 0, "resultEndTime"))
		assert.Equal(t, `"6s"`, compact(t, doc, "results", 1, "resultEndTime"))
		back := googleFormat.parse(t, orderedjson.Block(doc), transcript.ReadOptions{})
		assert.Equal(t, 2, back.Len())
	})

	t.Run("V1 results", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		s.SetMetadata(model.MetadataLanguage, "en-US")
		s.AddCue(model.Cue{Start: ms(1500), End: ms(3000), Lines: []string{"Good", "morning."}, Speaker: "Anna"})
		s.AddCue(model.Cue{
			Start: ms(4000), End: ms(5000), Lines: []string{"See you."},
			Words: []model.Word{{Text: "See", Start: ms(4000), End: ms(4500)}, {Text: "you.", Start: ms(4500), End: ms(5000)}},
		})
		want := `{"results":[{"alternatives":[{"transcript":"Good morning."}],"resultEndTime":"3s","languageCode":"en-US"},` +
			`{"alternatives":[{"transcript":"See you.","words":[{"startTime":"4s","endTime":"4.500s","word":"See"},` +
			`{"startTime":"4.500s","endTime":"5s","word":"you."}]}],"resultEndTime":"5s","languageCode":"en-US"}]}`
		assert.Equal(t, want, orderedjson.Block(googleFormat.output(t, s)))
	})
}

// A stored time that is no protobuf Duration for the writer, such as a numeric string or a plain number, is written
// again as a Duration string.
func TestWriteGoogleSpeechStoredTimesThatAreNoDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, start, end, resultEnd string
		wantStart, wantResultEnd    string
	}{
		{"a numeric string", `"0.66"`, `"1.5"`, `"1.5"`, `"0.660s"`, `"1.500s"`},
		{"a plain number", `0.66`, `1.5`, `1.5`, `"0.660s"`, `"1.500s"`},
		{"an exponent", `"1e0"`, `"2e0"`, `"2e0"`, `"1s"`, `"2s"`},
		{"a duration string", `"0.660s"`, `"1.500s"`, `"1.500s"`, `"0.660s"`, `"1.500s"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := `{"results": [{"alternatives": [{"transcript": "hi", "words": [{"startTime": ` + tt.start + `, "endTime": ` + tt.end +
				`, "word": "hi"}]}], "resultEndTime": ` + tt.resultEnd + `}]}`
			doc := googleFormat.output(t, googleFormat.parse(t, content, withWords))
			assert.Equal(t, tt.wantStart, compact(t, doc, "results", 0, "alternatives", 0, "words", 0, "startTime"))
			assert.Equal(t, tt.wantResultEnd, compact(t, doc, "results", 0, "resultEndTime"))
		})
	}
}
