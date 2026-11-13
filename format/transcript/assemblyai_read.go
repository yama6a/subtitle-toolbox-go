package transcript

import (
	"io"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const assemblyAIKey = string(model.AssemblyAI)

// ReadAssemblyAI reads the JSON of an AssemblyAI transcript from r. Its times are milliseconds. It makes one cue per
// utterance, else cues grouped from the words. A field error names the path of the field, such as words[3].start.
func ReadAssemblyAI(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	st, root, ok, err := readRoot(r, model.AssemblyAI, common)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	words, utterances := cloudGet(root, "words"), cloudGet(root, "utterances")
	hasUtterances := utterances.Kind() == orderedjson.Array && len(utterances.Items()) > 0
	if !hasUtterances && words.Kind() != orderedjson.Array {
		return nil, nil, &model.ParseError{Format: model.AssemblyAI, Msg: `The JSON has no "words" or "utterances" list.`}
	}
	cues := wordgroup.NewReader(st, wordgroup.Options{WordTimestamps: opts.WordTimestamps, SpeakerVoices: opts.SpeakerVoices}, assemblyAIKey)
	var list []*model.Cue
	if hasUtterances {
		list, err = cues.Utterances(utterances.Items(), "utterances", "text", time.Millisecond, func(items []orderedjson.Value, path string) ([]wordgroup.Word, error) {
			return cues.WordList(items, path, assemblyAIWord)
		})
	} else {
		var list2 []wordgroup.Word
		if list2, err = cues.WordList(words.Items(), "words", assemblyAIWord); err == nil {
			list, err = cues.CuesFromWords(list2, "words", nil)
		}
	}
	if err != nil {
		return nil, nil, err
	}
	s, err := cues.Transcript(list, assemblyAILanguage(root), cloudData(root.Without("text", "words", "utterances")))
	if err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func assemblyAIWord(item orderedjson.Value, path string) (wordgroup.Word, error) {
	text, err := wordgroup.Text(item, "text", path)
	if err != nil {
		return wordgroup.Word{}, err
	}
	start, err := wordgroup.Seconds(cloudGet(item, "start"), path+".start", time.Millisecond)
	if err != nil {
		return wordgroup.Word{}, err
	}
	end, err := wordgroup.Seconds(cloudGet(item, "end"), path+".end", time.Millisecond)
	if err != nil {
		return wordgroup.Word{}, err
	}
	w := wordgroup.Word{Text: strings.Trim(text, markupcore.PHPTrimSet), Start: start, Timed: true, End: end, HasEnd: true}
	w.Confidence = cloudConfidence(cloudGet(item, "confidence"))
	w.Speaker, w.HasSpeaker = wordgroup.Speaker(cloudGet(item, "speaker"))
	return w, nil
}

// assemblyAILanguage turns the language code of AssemblyAI, such as en_us, into a BCP 47 tag, such as en-US.
func assemblyAILanguage(root orderedjson.Value) orderedjson.Value {
	code, ok := cloudGet(root, "language_code").AsString()
	if !ok || code == "" {
		return orderedjson.Value{}
	}
	if lang, region, found := strings.Cut(code, "_"); found {
		code = lang + "-" + asciiUpper(region)
	}
	return orderedjson.StringValue(code)
}

func asciiUpper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'a' <= c && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}
