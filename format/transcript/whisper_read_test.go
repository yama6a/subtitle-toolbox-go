package transcript_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const twoSegmentsWithWords = `{
	"task": "transcribe", "language": "english", "duration": 8.47, "text": "The beach was quiet. Nobody came.",
	"segments": [
		{"id": 0, "start": 0.0, "end": 3.32, "text": " The beach was quiet."},
		{"id": 1, "start": 3.9, "end": 5.1, "text": " Nobody came."}
	],
	"words": [{"word": "The", "start": 0.0, "end": 0.24}, {"word": "beach", "start": 0.24, "end": 0.71}]
}`

func describe(s *model.Subtitle) []string {
	var out []string
	for _, c := range s.All() {
		out = append(out, fmt.Sprintf("%g-%g %s", c.Start.Seconds(), c.End.Seconds(), strings.Join(c.TaggedLines(), "|")))
	}
	return out
}

// Ports the cue tests of tests/Parsers/WhisperJsonParserTest.php of subtitle-toolbox-php.
func TestReadWhisperCues(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("The bus stops at every corner on the way to the station. ", 8)
	tests := []struct {
		name    string
		content string
		opts    transcript.ReadOptions
		want    []string
	}{
		{"one cue per segment", twoSegmentsWithWords, transcript.ReadOptions{}, []string{"0-3.32 The beach was quiet.", "3.9-5.1 Nobody came."}},
		{
			"word timestamps", twoSegmentsWithWords,
			transcript.ReadOptions{WordTimestamps: true},
			[]string{"0-3.32 <00:00:00.000>The <00:00:00.240>beach was quiet.", "3.9-5.1 Nobody came."},
		},
		{
			name: "words that the text lacks, and words without time",
			content: `{"segments": [{"start": 1, "end": 4, "text": " Tea, then toast <now> & later.", "words": [` +
				`{"word": " Tea,", "start": 1}, {"word": " coffee", "start": 1.5}, {"word": " then"}, ` +
				`{"word": " toast", "start": 2.25}, {"word": " <now>", "start": 3}, {"word": " &", "start": 3.5}]}]}`,
			opts: transcript.ReadOptions{WordTimestamps: true},
			want: []string{"1-4 <00:00:01.000>Tea, then <00:00:02.250>toast <00:00:03.000>&lt;now&gt; <00:00:03.500>&amp; later."},
		},
		{"escaped text", `{"segments": [{"start": 0, "end": 1, "text": " Fish & chips <3"}]}`, transcript.ReadOptions{}, []string{"0-1 Fish &amp; chips &lt;3"}},
		{
			"trimmed text, no cue for a blank segment, a BOM",
			"\xEF\xBB\xBF" + `{"segments": [{"start": 0, "end": 1, "text": " One. "}, {"start": 1, "end": 1, "text": ""}, ` +
				`{"start": 1, "end": 2, "text": "  "}, {"start": 2, "end": 3, "text": "Two."}]}`,
			transcript.ReadOptions{},
			[]string{"0-1 One.", "2-3 Two."},
		},
		{
			"a long segment stays one cue", `{"segments": [{"start": 0, "end": 30, "text": "` + long + `"}]}`,
			transcript.ReadOptions{},
			[]string{"0-30 " + strings.TrimSpace(long)},
		},
		{
			name: "whisper.cpp offsets and tokens",
			content: `{"result": {"language": "nl"}, "transcription": [{"timestamps": {"from": "00:00:01,000", "to": "00:00:03,500"}, ` +
				`"offsets": {"from": 1000, "to": 3500}, "text": " Good morning.", "tokens": [` +
				`{"text": "[_BEG_]", "offsets": {"from": 1000, "to": 1000}}, {"text": " Good", "offsets": {"from": 1000, "to": 1400}}, ` +
				`{"text": " mor", "offsets": {"from": 1400, "to": 1700}}, {"text": "ning", "offsets": {"from": 1700, "to": 2000}}, ` +
				`{"text": ".", "offsets": {"from": 2000, "to": 2100}}, {"text": "[_TT_125]", "offsets": {"from": 3500, "to": 3500}}]}]}`,
			opts: transcript.ReadOptions{WordTimestamps: true},
			want: []string{"1-3.5 <00:00:01.000>Good <00:00:01.400>morning."},
		},
		{
			"speakers: a string, a blank, a number and whisper.cpp",
			`{"segments": [{"start": 0, "end": 1, "text": "A", "speaker": "Anna"}, {"start": 1, "end": 2, "text": "B", "speaker": " "}, ` +
				`{"start": 2, "end": 3, "text": "C", "speaker": 7}]}`,
			transcript.ReadOptions{SpeakerVoices: true},
			[]string{"0-1 <v Anna>A", "1-2 B", "2-3 C"},
		},
		{
			"whisper.cpp speaker",
			`{"transcription": [{"offsets": {"from": 0, "to": 500}, "text": " Hi", "speaker": "?"}]}`,
			transcript.ReadOptions{SpeakerVoices: true},
			[]string{"0-0.5 <v ?>Hi"},
		},
		{
			"times as rounded numbers and a word that is not a string",
			`{"segments": [{"start": 0.0004, "end": 1.0006, "text": "A b", "words": [{"word": 5, "start": 0}, {"word": "b", "start": 0.5, "end": "x"}]}]}`,
			transcript.ReadOptions{WordTimestamps: true},
			[]string{"0-1.001 A <00:00:00.500>b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, describe(readWhisper(t, tt.content, tt.opts)))
		})
	}
}

func TestReadWhisperLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, content, want string }{
		{"API name", `{"language": "english", "segments": []}`, "en"},
		{"capitalised name", `{"language": "German", "segments": []}`, "de"},
		{"alias", `{"language": "castilian", "segments": []}`, "es"},
		{"code", `{"language": "fr", "segments": []}`, "fr"},
		{"three letters", `{"language": "yue", "segments": []}`, "yue"},
		{"unknown", `{"language": "Klingon", "segments": []}`, "Klingon"},
		{"whisper.cpp", `{"result": {"language": "japanese"}, "transcription": []}`, "ja"},
		{"language wins over the result", `{"language": "en", "result": {"language": "de"}, "segments": []}`, "en"},
		{"a name of 2 words", `{"language": "Haitian Creole", "segments": []}`, "ht"},
		{"not a string", `{"language": 5, "segments": []}`, ""},
		{"empty", `{"language": "", "segments": []}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, _ := readWhisper(t, tt.content, transcript.ReadOptions{}).Metadata(model.MetadataLanguage)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadWhisperFileData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"openai-whisper", `{"text": "a", "segments": [], "language": "en"}`, `{"format":"openai-whisper","language":"en"}`},
		{"faster-whisper with null words", `{"segments": [], "words": null, "task": null}`, `{"format":"openai-whisper","task":null}`},
		{"openai-api by task", `{"task": "transcribe", "segments": []}`, `{"format":"openai-api","task":"transcribe"}`},
		{"openai-api by words", `{"segments": [], "words": []}`, `{"format":"openai-api"}`},
		{"whisperx", `{"segments": [], "word_segments": [], "task": "x"}`, `{"format":"whisperx","task":"x"}`},
		{"whisper.cpp", `{"systeminfo": "s", "result": {"language": "en"}, "transcription": []}`, `{"format":"whisper.cpp","systeminfo":"s","result":{"language":"en"}}`},
		{"a format member is dropped", `{"format": "mine", "segments": []}`, `{"format":"openai-whisper"}`},
		{"segments win over transcription", `{"segments": [], "transcription": [1]}`, `{"format":"openai-whisper"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readWhisper(t, tt.content, transcript.ReadOptions{})
			d, _ := s.FormatData("whisper")
			var got strings.Builder
			got.WriteString("{")
			for i, k := range d.Keys() {
				if i > 0 {
					got.WriteString(",")
				}
				v, _ := d.Get(k)
				fmt.Fprintf(&got, "%q:%s", k, jsonOf(v))
			}
			got.WriteString("}")
			assert.Equal(t, tt.want, got.String())
		})
	}
}

// Ports the error rows of tests/Parsers/WhisperJsonParserTest.php, then adds the rows of the Go bounds.
func TestReadWhisperErrors(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, content, want string }{
		{"no JSON", "{", "The content is not valid JSON: Syntax error."},
		{"root list", "[1]", "The JSON root must be an object."},
		{"empty root list", "[]", "The JSON root must be an object."},
		{"only text", `{"text": "Hello"}`, `The JSON has no "segments" or "transcription" list.`},
		{"only words", `{"words": [{"word": "Hi", "start": 0, "end": 1}]}`, `The JSON has no "segments" or "transcription" list.`},
		{"segments as an object", `{"segments": {"0": {"start": 0, "end": 1, "text": "a"}}}`, `The JSON has no "segments" or "transcription" list.`},
		{"start no number", `{"segments": [{"start": "0", "end": 1, "text": "Hi"}]}`, "The field segments[0].start must be a number."},
		{"segment no object", `{"segments": [5]}`, "The field segments[0].start must be a number."},
		{"no text", `{"segments": [{"start": 0, "end": 1}]}`, "The field segments[0].text must be a string."},
		{"text a number", `{"segments": [{"start": 0, "end": 1, "text": 5}]}`, "The field segments[0].text must be a string."},
		{"whisper.cpp offsets", `{"transcription": [{"offsets": {"from": 0}, "text": "Hi"}]}`, "The field transcription[0].offsets.to must be a number."},
		{"whisper.cpp no offsets", `{"transcription": [{"text": "Hi"}]}`, "The field transcription[0].offsets.from must be a number."},
		{"negative start", `{"segments": [{"id": 0, "start": -0.5, "end": 1.0, "text": "Hello there"}]}`, "The field segments[0].start must be a time of 0 or more."},
		{"negative offsets", `{"transcription": [{"offsets": {"from": -500, "to": 1000}, "text": "Hi"}]}`, "The field transcription[0].offsets.from must be a time of 0 or more."},
		{"a number out of range", `{"segments": [{"start": 0, "end": 1e400, "text": "Hi"}]}`, "The field segments[0].end must be a number."},
		{"start of 100000 hours", `{"segments": [{"start": 360000000, "end": 360000001, "text": "Hi"}]}`, "The field segments[0].start is not below 100000 hours."},
		{"start that rounds to 100000 hours", `{"segments": [{"start": 359999999.9996, "end": 360000000, "text": "Hi"}]}`, "The field segments[0].start is not below 100000 hours."},
		{"offset of 100000 hours", `{"transcription": [{"offsets": {"from": 0, "to": 360000000000}, "text": "Hi"}]}`, "The field transcription[0].offsets.to is not below 100000 hours."},
		{
			"word start of 100000 hours", `{"segments": [{"start": 0, "end": 1, "text": "a", "words": [{"word": "a", "start": 1e12}]}]}`,
			"The field segments[0].words[0].start is not below 100000 hours.",
		},
		{
			"top-level word start of 100000 hours",
			`{"segments": [{"start": 0, "end": 1, "text": "a"}], "words": [{"word": "a", "start": 0}, {"word": "b", "start": 1e12}]}`,
			"The field words[1].start is not below 100000 hours.",
		},
		{
			"token start of 100000 hours",
			`{"transcription": [{"offsets": {"from": 0, "to": 1}, "text": "a", "tokens": [{"text": "[_BEG_]"}, {"text": " a", "offsets": {"from": 1e15}}]}]}`,
			"The field transcription[0].tokens[1].offsets.from is not below 100000 hours.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := transcript.ReadWhisper(strings.NewReader(tt.content), model.CommonReadOptions{}, transcript.ReadOptions{})
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.want, perr.Msg)
			assert.Equal(t, model.Whisper, perr.Format)
		})
	}
}

func TestReadWhisperOutOfRangeFile(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, "whisper/own_out_of_range_number.json"))
	_, _, err := transcript.ReadWhisper(strings.NewReader(content), model.CommonReadOptions{}, transcript.ReadOptions{})
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, "The field segments[1].start must be a number.", perr.Msg)

	s, warnings, err := transcript.ReadWhisper(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, transcript.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"0-2.84 La boulangerie ouvre \u00e0 sept heures."}, describe(s))
	require.Len(t, warnings, 1)
	assert.Equal(t, 1, warnings[0].Block)
	assert.Equal(t, model.Skipped, warnings[0].Action)
	assert.Contains(t, warnings[0].Lines[0], `"avg_logprob":1e400,`)
}

func TestReadWhisperLenient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string
		opts         transcript.ReadOptions
		wantCues     []string
		wantWarnings []string
	}{
		{
			name: "a segment that fails gives its top-level words to the next segment",
			content: `{"segments": [{"start": 0, "end": 1, "text": "a"}, {"start": 1, "text": "b"}, {"start": 2, "end": 3, "text": "c d"}],` +
				` "words": [{"word": "a", "start": 0, "end": 0.5}, {"word": "c", "start": 2, "end": 2.5}, {"word": "d", "start": 2.5, "end": 3}]}`,
			opts:     transcript.ReadOptions{WordTimestamps: true},
			wantCues: []string{"0-1 <00:00:00.000>a", "2-3 <00:00:02.000>c <00:00:02.500>d"},
			wantWarnings: []string{
				`1 The field segments[1].end must be a number. {"start":1,"text":"b"}`,
			},
		},
		{
			name:     "a word that fails skips the segment with its words",
			content:  `{"segments": [{"start": 0, "end": 1, "text": "a", "words": [{"word": "a", "start": 1e12}]}, {"start": 1, "end": 2, "text": "b"}]}`,
			wantCues: []string{"1-2 b"},
			wantWarnings: []string{
				`0 The field segments[0].words[0].start is not below 100000 hours. {"start":0,"end":1,"text":"a","words":[{"word":"a","start":1e12}]}`,
			},
		},
		{
			name:         "an end before the start is swapped",
			content:      `{"segments": [{"start": 2, "end": 1, "text": "a"}]}`,
			wantCues:     []string{"1-2 a"},
			wantWarnings: []string{`0 The cue ends at 1 s, before it starts at 2 s. The parser swapped the times. {"start":2,"end":1,"text":"a"}`},
		},
		{
			name:     "a transcription entry that fails is skipped",
			content:  `{"transcription": [{"offsets": {"from": 0}, "text": "a"}, {"offsets": {"from": 0, "to": 5}, "text": "b"}]}`,
			wantCues: []string{"0-0.005 b"},
			wantWarnings: []string{
				`0 The field transcription[0].offsets.to must be a number. {"offsets":{"from":0},"text":"a"}`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := transcript.ReadWhisper(strings.NewReader(tt.content), model.CommonReadOptions{Lenient: true}, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCues, describe(s))
			var got []string
			for _, w := range warnings {
				got = append(got, fmt.Sprintf("%d %s %s", w.Block, w.Message, w.Lines[0]))
			}
			assert.Equal(t, tt.wantWarnings, got)
		})
	}
}

func TestReadWhisperTopLevelWords(t *testing.T) {
	t.Parallel()

	segments := `{"start": 0, "end": 1, "text": "a b"}, {"start": 1, "end": 2, "text": "c d"}, {"start": 2, "end": 3, "text": "e"}`
	tests := []struct {
		name  string
		words string
		want  []string
	}{
		{
			"by the middle of each word", `{"word": "a", "start": 0, "end": 0.5}, {"word": "b", "start": 0.5, "end": 1}, ` +
				`{"word": "c", "start": 1, "end": 1.5}, {"word": "d", "start": 1.5, "end": 2}, {"word": "e", "start": 2, "end": 3}`,
			[]string{"a b", "c d", "e"},
		},
		{"a word on the boundary stays", `{"word": "a", "start": 0.5, "end": 1.5}, {"word": "b", "start": 1.5, "end": 2.5}`, []string{"a", "b", ""}},
		{"strings as times, a word without time and an end that defaults", `{"word": "a", "start": "0.2", "end": "0.4"}, {"word": "b"}, {"word": "c", "start": 1.4}`, []string{"a b", "c", ""}},
		{"the last segment takes the rest", `{"word": "a", "start": 9, "end": 10}`, []string{"", "", "a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readWhisper(t, `{"segments": [`+segments+`], "words": [`+tt.words+`]}`, transcript.ReadOptions{})
			var got []string
			for _, c := range s.All() {
				data, _ := c.FormatData("whisper")
				var texts []string
				words, _ := data.List("words")
				for _, w := range words {
					d, _ := w.(*model.Data)
					text, _ := d.String("word")
					texts = append(texts, text)
				}
				got = append(got, strings.Join(texts, " "))
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadWhisperWordFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		word string
		want model.Word
	}{
		{"end and probability", `{"word": " a", "start": 1, "end": 1.5, "probability": 0.25}`, model.Word{Text: "a", Start: ms(1000), End: ms(1500), Confidence: new(0.25)}},
		{"score when probability is missing", `{"word": "a", "start": 1, "end": 1.5, "score": 0.5}`, model.Word{Text: "a", Start: ms(1000), End: ms(1500), Confidence: new(0.5)}},
		{"score when probability is null", `{"word": "a", "start": 1, "end": 1.5, "probability": null, "score": 0.5}`, model.Word{Text: "a", Start: ms(1000), End: ms(1500), Confidence: new(0.5)}},
		{"a confidence over 1 is no confidence", `{"word": "a", "start": 1, "end": 1.5, "probability": 1.5, "score": 0.5}`, model.Word{Text: "a", Start: ms(1000), End: ms(1500)}},
		{"an end from the cue end", `{"word": "a", "start": 1}`, model.Word{Text: "a", Start: ms(1000), End: ms(3000)}},
		{"an end before the start", `{"word": "a", "start": 1, "end": 0.5}`, model.Word{Text: "a", Start: ms(1000), End: ms(1000)}},
		{"an end of 100000 hours is no end", `{"word": "a", "start": 1, "end": 1e12}`, model.Word{Text: "a", Start: ms(1000), End: ms(3000)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readWhisper(t, `{"segments": [{"start": 0, "end": 3, "text": "a", "words": [`+tt.word+`]}]}`, transcript.ReadOptions{WordTimestamps: true})
			c, _ := s.Cue(0)
			assert.Equal(t, []model.Word{tt.want}, c.Words)
		})
	}
}

func TestReadWhisperWithoutWordTimestampsKeepsTheWords(t *testing.T) {
	t.Parallel()

	s := readFile(t, "openai_whisper_word_timestamps.json", transcript.ReadOptions{})
	c, _ := s.Cue(0)
	assert.Empty(t, c.Words)
	data, _ := c.FormatData("whisper")
	words, _ := data.List("words")
	assert.Len(t, words, 5)
}

func TestReadWhisperReadError(t *testing.T) {
	t.Parallel()

	_, _, err := transcript.ReadWhisper(errReader{}, model.CommonReadOptions{}, transcript.ReadOptions{})
	require.Error(t, err)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }
