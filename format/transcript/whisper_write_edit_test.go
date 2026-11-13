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

// check is one expectation: the JSON text at a path of the written document.
type check struct {
	path []any
	want string
}

// Ports the editing tests of tests/Formatters/WhisperJsonFormatterTest.php of subtitle-toolbox-php. A test that edited
// word timestamp tags in the lines edits Cue.Words here.
func TestWriteWhisperEdits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
		opts transcript.ReadOptions
		// index is the cue that edit changes. add is a cue that the test appends.
		index  int
		edit   func(c *model.Cue)
		add    *model.Cue
		checks []check
		// absent lists paths that must not exist.
		absent [][]any
	}{
		{
			name: "an edited word gets confidence 1.0 and the other words keep theirs",
			file: "openai_whisper_word_timestamps.json", opts: withWords,
			index: 0,
			edit: func(c *model.Cue) {
				c.Lines = []string{"The baker opens at seven."}
				c.Words[1].Text = "baker"
			},
			checks: []check{
				{[]any{"segments", 0, "text"}, `" The baker opens at seven."`},
				{[]any{"segments", 0, "words", 1}, `{"word":" baker","start":0.24,"end":0.7,"probability":1.0}`},
				{[]any{"segments", 0, "words", 0}, `{"word":" The","start":0.0,"end":0.24,"probability":0.991234540939331}`},
				{[]any{"segments", 0, "words", 2, "probability"}, `0.9876543283462524`},
			},
		},
		{
			name:  "a word changed without word timestamps keeps the times of its source word",
			file:  "openai_whisper_word_timestamps.json",
			index: 0,
			edit:  func(c *model.Cue) { c.Lines = []string{"The bakery opens at six."} },
			checks: []check{
				{[]any{"segments", 0, "text"}, `" The bakery opens at six."`},
				{[]any{"segments", 0, "words", 4}, `{"word":" six.","start":1.32,"end":2.2,"probability":1.0}`},
				{[]any{"segments", 0, "words", 1, "probability"}, `0.9534912109375`},
				{[]any{"segments", 1, "tokens"}, `[50474,440,37633,20402,33724,293,272,2953,13,50594]`},
			},
			absent: [][]any{{"segments", 0, "tokens"}},
		},
		{
			name:   "a text with other words and no word timestamps has no words",
			file:   "openai_whisper_word_timestamps.json",
			index:  0,
			edit:   func(c *model.Cue) { c.Lines = []string{"The bakery opens at six today."} },
			checks: []check{{[]any{"segments", 0, "start"}, `0.0`}},
			absent: [][]any{{"segments", 0, "words"}},
		},
		{
			name:  "a moved cue writes its new times",
			file:  "openai_api_verbose_json_french.json",
			index: 1,
			edit:  func(c *model.Cue) { c.Start, c.End = ms(3000), ms(5500) },
			checks: []check{
				{[]any{"segments", 1, "start"}, `3.0`},
				{[]any{"segments", 1, "end"}, `5.5`},
				{[]any{"segments", 0, "end"}, `2.8399999141693115`},
			},
		},
		{
			name: "the OpenAI API shape lists the words at the top level",
			file: "openai_api_verbose_json_words.json", opts: withWords,
			index: 1,
			edit: func(c *model.Cue) {
				c.Lines = []string{"Somebody came."}
				c.Words = []model.Word{{Text: "Somebody", Start: ms(4320), End: ms(4900)}, {Text: "came.", Start: ms(4900), End: ms(5100)}}
			},
			checks: []check{
				{[]any{"text"}, `"The beach was quiet in the morning. Somebody came. The tide was low at noon."`},
				{[]any{"words", 7}, `{"word":"Somebody","start":4.32,"end":4.9}`},
				{[]any{"words", 8}, `{"word":"came.","start":4.9,"end":5.1}`},
			},
			absent: [][]any{{"segments", 1, "words"}},
		},
		{
			name: "a new speaker changes the speaker of the segment and of its words",
			file: "whisperx_diarize.json", opts: withWords,
			index: 0,
			edit:  func(c *model.Cue) { c.Speaker = "SPEAKER_02" },
			checks: []check{
				{[]any{"segments", 0, "speaker"}, `"SPEAKER_02"`},
				{[]any{"segments", 0, "words", 0, "speaker"}, `"SPEAKER_02"`},
				{[]any{"segments", 0, "words", 4, "speaker"}, `"SPEAKER_02"`},
				{[]any{"word_segments", 0, "speaker"}, `"SPEAKER_02"`},
				{[]any{"segments", 1, "speaker"}, `"SPEAKER_01"`},
			},
		},
		{
			name: "an edited whisper.cpp entry has no tokens",
			file: "whisper_cpp_ojf.json", opts: withWords,
			index: 0,
			edit: func(c *model.Cue) {
				c.Lines = []string{"The museum opens at nine."}
				c.Words = nil
			},
			checks: []check{{[]any{"transcription", 0, "text"}, `" The museum opens at nine."`}},
			absent: [][]any{{"transcription", 0, "tokens"}},
		},
		{
			name:  "a moved whisper.cpp entry gets new offsets and timestamps",
			file:  "whisper_cpp_oj.json",
			index: 0,
			edit:  func(c *model.Cue) { c.End = ms(62500) },
			checks: []check{
				{[]any{"transcription", 0, "offsets"}, `{"from":0,"to":62500}`},
				{[]any{"transcription", 0, "timestamps"}, `{"from":"00:00:00,000","to":"00:01:02,500"}`},
			},
		},
		{
			name:   "a new speaker in a whisper.cpp entry",
			file:   "whisper_cpp_oj.json",
			index:  0,
			edit:   func(c *model.Cue) { c.Speaker = "Zoe" },
			checks: []check{{[]any{"transcription", 0, "speaker"}, `"Zoe"`}},
			absent: [][]any{{"transcription", 1, "speaker"}},
		},
		{
			name: "D1: a reused word gets the typed end and confidence",
			file: "openai_whisper_word_timestamps.json", opts: withWords,
			index: 0,
			edit: func(c *model.Cue) {
				c.Words[1].End = ms(650)
				c.Words[1].Confidence = new(0.5)
			},
			checks: []check{
				{[]any{"segments", 0, "words", 1}, `{"word":" bakery","start":0.24,"end":0.65,"probability":0.5}`},
				{[]any{"segments", 0, "words", 2, "probability"}, `0.9876543283462524`},
			},
		},
		{
			name:   "a cue that is not in the source gets the id after the id before it",
			file:   "faster_whisper_ctranslate2_segments.json",
			add:    &model.Cue{Start: ms(7000), End: ms(8000), Lines: []string{"New."}},
			checks: []check{{[]any{"segments", 2}, `{"id":3,"start":7.0,"end":8.0,"text":" New."}`}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, tt.file, tt.opts)
			if tt.edit != nil {
				editCue(t, s, tt.index, tt.edit)
			}
			if tt.add != nil {
				s.AddCue(*tt.add)
			}
			doc := writeWhisper(t, s)
			for _, c := range tt.checks {
				assert.Equal(t, c.want, compact(t, doc, c.path...), "%v", c.path)
			}
			for _, p := range tt.absent {
				last, parent := p[len(p)-1].(string), compact(t, doc, p[:len(p)-1]...)
				assert.NotContains(t, parent, `"`+last+`":`, "%v", p)
			}
		})
	}
}

// Ports testCuesFromAnotherFormatGiveTheShapeOfOpenAiWhisper.
func TestWriteWhisperFromAnotherFormat(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataLanguage, "de")
	s.AddCue(model.Cue{Start: ms(1500), End: ms(3000), Lines: []string{"<v Anna>Guten", "Morgen."}})
	s.AddCue(model.Cue{
		Start: ms(4000), End: ms(5000), Lines: []string{"Bis bald."},
		Words: []model.Word{{Text: "Bis", Start: ms(4000), End: ms(4500)}, {Text: "bald.", Start: ms(4500), End: ms(5000)}},
	})
	var out bytes.Buffer
	require.NoError(t, transcript.WriteWhisper(&out, s, model.CommonWriteOptions{}, transcript.WriteOptions{}))
	want := `{"text":" Guten Morgen. Bis bald.","segments":[{"id":0,"start":1.5,"end":3.0,"text":" Guten Morgen.","speaker":"Anna"},` +
		`{"id":1,"start":4.0,"end":5.0,"text":" Bis bald.","words":[{"word":" Bis","start":4.0,"end":4.5,"probability":1.0},` +
		`{"word":" bald.","start":4.5,"end":5.0,"probability":1.0}]}],"language":"de"}`
	assert.Equal(t, want, out.String())
}

func TestWriteWhisperShapeAndLanguage(t *testing.T) {
	t.Parallel()

	cue := model.Cue{Start: ms(1000), End: ms(2000), Lines: []string{"Hi."}}
	tests := []struct {
		name     string
		data     string
		language string
		want     string
	}{
		{"the stored shape wins", `{"format":"whisper.cpp","systeminfo":"x"}`, "en", `{"systeminfo":"x","transcription":[{"timestamps":{"from":"00:00:01,000","to":"00:00:02,000"},"offsets":{"from":1000,"to":2000},"text":" Hi."}]}`},
		{"an unknown shape gives openai-whisper", `{"format":"other"}`, "en", `{"text":" Hi.","segments":[{"id":0,"start":1.0,"end":2.0,"text":" Hi."}],"language":"en"}`},
		{"a stored language wins", `{"format":"openai-api","language":"english"}`, "en", `{"language":"english","text":"Hi.","segments":[{"id":0,"start":1.0,"end":2.0,"text":" Hi."}]}`},
		{"a null language is replaced in place", `{"language":null,"x":1}`, "en", `{"text":" Hi.","segments":[{"id":0,"start":1.0,"end":2.0,"text":" Hi."}],"language":"en","x":1}`},
		{"whisperx has no ids", `{"format":"whisperx"}`, "", `{"segments":[{"start":1.0,"end":2.0,"text":" Hi."}],"word_segments":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &model.Subtitle{}
			s.AddCue(cue)
			if tt.language != "" {
				s.SetMetadata(model.MetadataLanguage, tt.language)
			}
			data := model.NewData()
			v := parseJSON(t, tt.data)
			for _, m := range v.Members() {
				if str, ok := m.Value.AsString(); ok && m.Value.Kind() == orderedjson.String {
					data.Set(m.Key, str)
				} else if n, ok := m.Value.AsInt64(); ok {
					data.Set(m.Key, n)
				} else {
					data.Set(m.Key, nil)
				}
			}
			require.NoError(t, s.SetFormatData("whisper", data))
			var out bytes.Buffer
			require.NoError(t, transcript.WriteWhisper(&out, s, model.CommonWriteOptions{}, transcript.WriteOptions{}))
			assert.Equal(t, tt.want, out.String())
		})
	}
}

func TestWriteWhisperErrors(t *testing.T) {
	t.Parallel()

	t.Run("a number that JSON cannot hold", func(t *testing.T) {
		t.Parallel()

		s := readWhisper(t, `{"segments": [{"start": 0, "end": 1, "text": "Hi", "avg_logprob": 1e400}]}`, transcript.ReadOptions{})
		err := transcript.WriteWhisper(&bytes.Buffer{}, s, model.CommonWriteOptions{}, transcript.WriteOptions{})
		var uerr *model.UnwritableError
		require.ErrorAs(t, err, &uerr)
		assert.Equal(t, "Whisper JSON cannot hold the subtitle: Inf and NaN cannot be JSON encoded.", uerr.Reason)
	})
	t.Run("format data that is not UTF-8", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		s.AddCue(model.Cue{End: ms(1), Lines: []string{"a"}})
		require.NoError(t, s.SetFormatData("whisper", model.NewData(model.Field{Key: "x", Value: "a\xffb"})))
		err := transcript.WriteWhisper(&bytes.Buffer{}, s, model.CommonWriteOptions{}, transcript.WriteOptions{})
		require.ErrorIs(t, err, model.ErrInvalidUTF8)
	})
	t.Run("an empty subtitle", func(t *testing.T) {
		t.Parallel()

		var out bytes.Buffer
		require.NoError(t, transcript.WriteWhisper(&out, &model.Subtitle{}, model.CommonWriteOptions{}, transcript.WriteOptions{}))
		assert.JSONEq(t, `{"text":"","segments":[]}`, out.String())
	})
}
