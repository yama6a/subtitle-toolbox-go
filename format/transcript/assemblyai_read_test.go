package transcript_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var assemblyAIFormat = cloudFormat{folder: "assemblyai", read: transcript.ReadAssemblyAI, write: transcript.WriteAssemblyAI}

// assemblyAIWords returns a transcript of words with the text, the start and the end in milliseconds, and a speaker.
func assemblyAIWords(words ...[]any) string {
	list := make([]string, len(words))
	for i, w := range words {
		speaker := "null"
		if len(w) > 3 {
			speaker = fmt.Sprintf("%q", w[3])
		}
		list[i] = fmt.Sprintf(`{"text": %q, "start": %d, "end": %d, "confidence": 0.9, "speaker": %s}`, w[0], w[1], w[2], speaker)
	}
	return `{"words": [` + strings.Join(list, ",") + `]}`
}

// Ports testGroupsWordsIntoCues, testWritesTheSpeakerAsVoiceTagOnlyWithTheOption, testEscapesTextAndKeepsItsMarkup and
// testAssemblyAiConvertsTheLanguageCodeToBcp47 of tests/Parsers/CloudSpeechParsersTest.php of subtitle-toolbox-php.
func TestReadAssemblyAICues(t *testing.T) {
	t.Parallel()

	w := func(text string, start, end int, speaker ...string) []any {
		if len(speaker) > 0 {
			return []any{text, start, end, speaker[0]}
		}
		return []any{text, start, end}
	}
	tests := []struct {
		name  string
		words [][]any
		opts  transcript.ReadOptions
		want  []string
	}{
		{"sentence end", [][]any{w("One.", 0, 300), w("Two", 400, 700)}, transcript.ReadOptions{}, []string{"0-0.3 One.", "0.4-0.7 Two"}},
		{"question mark and quote", [][]any{w("Why?\"", 0, 300), w("Two", 400, 700)}, transcript.ReadOptions{}, []string{"0-0.3 Why?\"", "0.4-0.7 Two"}},
		{"lower case after a dot", [][]any{w("e.g.", 0, 300), w("this", 400, 700)}, transcript.ReadOptions{}, []string{"0-0.7 e.g. this"}},
		{"CJK full stop", [][]any{w("\u3067\u3059\u3002", 0, 300), w("\u306f\u3044", 400, 700)}, transcript.ReadOptions{}, []string{"0-0.3 \u3067\u3059\u3002", "0.4-0.7 \u306f\u3044"}},
		{"gap of 1 s", [][]any{w("wait", 0, 300), w("then", 1300, 1600)}, transcript.ReadOptions{}, []string{"0-0.3 wait", "1.3-1.6 then"}},
		{"gap below 1 s", [][]any{w("wait", 0, 300), w("then", 1299, 1600)}, transcript.ReadOptions{}, []string{"0-1.6 wait then"}},
		{"speaker change", [][]any{w("Hi", 0, 300, "A"), w("there", 400, 700, "B")}, transcript.ReadOptions{}, []string{"0-0.3 Hi", "0.4-0.7 there"}},
		{
			"84 characters",
			[][]any{w(strings.Repeat("a", 41), 0, 300), w(strings.Repeat("b", 42), 300, 600), w("c", 600, 900)},
			transcript.ReadOptions{},
			[]string{"0-0.6 " + strings.Repeat("a", 41) + " " + strings.Repeat("b", 42), "0.6-0.9 c"},
		},
		{
			"84 characters in UTF-8",
			[][]any{w(strings.Repeat("\u00e9", 41), 0, 300), w(strings.Repeat("\u00e8", 42), 300, 600)},
			transcript.ReadOptions{},
			[]string{"0-0.6 " + strings.Repeat("\u00e9", 41) + " " + strings.Repeat("\u00e8", 42)},
		},
		{
			"84 characters with marks",
			[][]any{w(strings.Repeat("e\u0301", 41), 0, 300), w(strings.Repeat("a\u0308", 42), 300, 600), w("c", 600, 900)},
			transcript.ReadOptions{},
			[]string{"0-0.6 " + strings.Repeat("e\u0301", 41) + " " + strings.Repeat("a\u0308", 42), "0.6-0.9 c"},
		},
		{
			"the speaker as a voice only with the option",
			[][]any{w("Hi.", 0, 300, "O'Neil"), w("Bye.", 400, 700, "B")},
			transcript.ReadOptions{SpeakerVoices: true},
			[]string{"0-0.3 <v O'Neil>Hi.", "0.4-0.7 <v B>Bye."},
		},
		{"no voice by default", [][]any{w("Hi.", 0, 300, "O'Neil")}, transcript.ReadOptions{}, []string{"0-0.3 Hi."}},
		{"escaped text", [][]any{w("<b>", 0, 300), w("&", 300, 600)}, transcript.ReadOptions{}, []string{"0-0.6 &lt;b&gt; &amp;"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, describe(assemblyAIFormat.parse(t, assemblyAIWords(tt.words...), tt.opts)))
		})
	}
}

func TestReadAssemblyAIUtterancesAndLanguage(t *testing.T) {
	t.Parallel()

	s := assemblyAIFormat.parse(t, `{"language_code": "en_us", "text": "x", "audio_url": "u", "words": [], "utterances": [`+
		`{"start": 0, "end": 2000, "text": "Hi there.", "speaker": "A", "words": [`+
		`{"text": "Hi", "start": 0, "end": 1000, "speaker": "A"}, {"text": "there.", "start": 1000, "end": 2000, "speaker": "A"}]}]}`,
		transcript.ReadOptions{WordTimestamps: true, SpeakerVoices: true})
	assert.Equal(t, []string{"0-2 <v A><00:00:00.000>Hi <00:00:01.000>there."}, describe(s))
	file, _ := s.FormatData("assemblyai")
	assert.Equal(t, []string{"language_code", "audio_url"}, file.Keys())

	for code, want := range map[string]string{"en_us": "en-US", "en_au": "en-AU", "de": "de", "zh_cn_x": "zh-CN_X", "": "", "en_": "en-"} {
		s := assemblyAIFormat.parse(t, `{"language_code": "`+code+`", "words": []}`, transcript.ReadOptions{})
		got, _ := s.Metadata(model.MetadataLanguage)
		assert.Equal(t, want, got, code)
	}
}

func TestReadAssemblyAIErrors(t *testing.T) {
	t.Parallel()

	assemblyAIFormat.testBroken(t, []cloudBroken{
		{
			"text", `{"words": [{"text": null}, {"text": "there", "start": 1000, "end": 2000}]}`,
			"The field words[0].text must be a string.",
			[]string{"1-2 there"},
			0,
		},
		{
			"start", `{"words": [{"text": "hi", "start": -1, "end": 5}, {"text": "there", "start": 1000, "end": 2000}]}`,
			"The field words[0].start must be a time.",
			[]string{"1-2 there"},
			0,
		},
		{
			"utterance", `{"words": [], "utterances": [{"start": 0, "text": "Hi"}, {"start": 1000, "end": 2000, "text": "Bye"}]}`,
			"The field utterances[0].end must be a time.",
			[]string{"1-2 Bye"},
			0,
		},
		{
			"a word of an utterance keeps the cue",
			`{"utterances": [{"start": 0, "end": 1000, "text": "Hi", "words": [{"text": "Hi", "start": "x"}]}, {"start": 1000, "end": 2000, "text": "Bye"}]}`,
			"The field utterances[0].words[0].start must be a time.",
			[]string{"0-1 Hi", "1-2 Bye"},
			0,
		},
	})

	for _, content := range []string{`{}`, `{"words": {"0": {}}}`, `{"words": null, "utterances": []}`, `{"words": "x", "utterances": null}`} {
		_, _, err := transcript.ReadAssemblyAI(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, transcript.ReadOptions{})
		var perr *model.ParseError
		require.ErrorAs(t, err, &perr, content)
		assert.Equal(t, `The JSON has no "words" or "utterances" list.`, perr.Msg)
	}
}
