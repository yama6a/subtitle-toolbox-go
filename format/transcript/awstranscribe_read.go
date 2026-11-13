package transcript

import (
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const awsKey = string(model.AWSTranscribe)

// ReadAWSTranscribe reads the JSON of an Amazon Transcribe batch job from r. It makes one cue per audio segment,
// else cues grouped from the words. A field error names the path of the field, such as results.items[3].start_time.
func ReadAWSTranscribe(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	st, root, ok, err := readRoot(r, model.AWSTranscribe, common)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	results, _ := root.Get("results")
	items, _ := results.Get("items")
	if results.Kind() != orderedjson.Object || items.Kind() != orderedjson.Array {
		return nil, nil, &model.ParseError{Format: model.AWSTranscribe, Msg: `The JSON has no "results.items" list.`}
	}
	rd := &awsReader{st: st, cues: wordgroup.NewReader(st, wordgroup.Options{
		WordTimestamps: opts.WordTimestamps, SpeakerVoices: opts.SpeakerVoices,
	}, awsKey)}
	words, err := rd.words(items.Items())
	if err != nil {
		return nil, nil, err
	}
	cues, err := rd.cuesOf(results, words)
	if err != nil {
		return nil, nil, err
	}
	if labels, _ := results.Get("speaker_labels"); labels.Kind() == orderedjson.Object {
		results = results.With("speaker_labels", labels.Without("segments"))
	}
	language, _ := results.Get("language_code")
	fileData := wordgroup.WithResults(root, results, "transcripts", "items", "audio_segments", "channel_labels")
	s, err := rd.cues.Transcript(cues, language, cloudData(fileData))
	if err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

type awsReader struct {
	st   *parsing.State
	cues *wordgroup.Reader
}

// cuesOf makes one cue for each audio segment, or groups the words when there are no segments.
func (rd *awsReader) cuesOf(results orderedjson.Value, words []wordgroup.Word) ([]*model.Cue, error) {
	if segments, _ := results.Get("audio_segments"); segments.Kind() == orderedjson.Array && len(segments.Items()) > 0 {
		return rd.segments(segments.Items(), words)
	}
	return rd.cues.CuesFromWords(words, "items", nil)
}

// awsWord is a word under construction: a pronunciation item with the punctuation items around it.
type awsWord struct {
	parts  []string
	source []orderedjson.Value
	word   wordgroup.Word
}

func (w *awsWord) add(content string, item orderedjson.Value) {
	w.parts = append(w.parts, content)
	w.source = append(w.source, item)
}

func (w *awsWord) finish() wordgroup.Word {
	w.word.Text = strings.Trim(strings.Join(w.parts, ""), markupcore.PHPTrimSet)
	w.word.Source = w.source
	return w.word
}

// words joins each punctuation item to the word before it. Punctuation before the first word waits for it.
func (rd *awsReader) words(items []orderedjson.Value) ([]wordgroup.Word, error) {
	var words []wordgroup.Word
	var current *awsWord
	var waiting awsWord
	for i, item := range items {
		content, w, punctuation, err := awsReadItem(item, "results.items["+strconv.Itoa(i)+"]")
		if err != nil {
			if err = cloudFail(rd.st, err, i, item); err != nil {
				return nil, err
			}
			continue
		}
		switch {
		case punctuation && current == nil:
			waiting.add(content, item)
		case punctuation:
			current.add(content, item)
		default:
			if current != nil {
				words = append(words, current.finish())
			}
			current = &awsWord{parts: waiting.parts, source: waiting.source, word: w}
			current.add(content, item)
			waiting = awsWord{}
		}
	}
	if current != nil {
		words = append(words, current.finish())
	}
	return words, nil
}

// awsReadItem returns the content of an item and whether it is punctuation. A pronunciation item also gives its word
// without the text.
func awsReadItem(item orderedjson.Value, path string) (content string, w wordgroup.Word, punctuation bool, err error) {
	var first orderedjson.Value
	if alternatives, _ := item.Get("alternatives"); len(alternatives.Items()) > 0 {
		first = alternatives.Items()[0]
	}
	if content, err = wordgroup.Text(first, "content", path+".alternatives[0]"); err != nil || awsIsPunctuation(item) {
		return content, w, err == nil, err
	}
	v, _ := item.Get("start_time")
	start, err := wordgroup.Seconds(v, path+".start_time", time.Second)
	if err != nil {
		return content, w, false, err
	}
	v, _ = item.Get("end_time")
	end, err := wordgroup.Seconds(v, path+".end_time", time.Second)
	if err != nil {
		return content, w, false, err
	}
	w = wordgroup.Word{Start: start, Timed: true, End: end, HasEnd: true}
	label, _ := item.Get("speaker_label")
	w.Speaker, w.HasSpeaker = wordgroup.Speaker(label)
	conf, _ := first.Get("confidence")
	w.Confidence = awsConfidence(conf)
	return content, w, false, nil
}

// awsConfidence returns a number or a numeric string from 0 to 1.
func awsConfidence(v orderedjson.Value) *float64 {
	var f float64
	switch v.Kind() {
	case orderedjson.Number:
		f, _ = v.AsFloat64()
	case orderedjson.String:
		s, _ := v.AsString()
		text, ok := orderedjson.NumericString(s)
		if !ok {
			return nil
		}
		f, _ = strconv.ParseFloat(text, 64)
	case orderedjson.Null, orderedjson.Bool, orderedjson.Array, orderedjson.Object:
		return nil
	}
	if math.IsNaN(f) || f < 0 || f > 1 {
		return nil
	}
	return &f
}

// idKey returns the key under which an item id is known, or false for an id that is no integer or string.
// The integer 5 and the string "5" give one key, as PHP array keys do.
func idKey(id orderedjson.Value) (string, bool) {
	if i, ok := id.AsInt64(); ok {
		return strconv.FormatInt(i, 10), true
	}
	s, ok := id.AsString()
	if !ok {
		return "", false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil && strconv.FormatInt(i, 10) == s {
		return s, true
	}
	return "s:" + s, true
}

// awsWordIDs maps each item id to the index of the word that holds the item.
func awsWordIDs(words []wordgroup.Word) map[string]int {
	byID := map[string]int{}
	for k, w := range words {
		for _, item := range w.Source {
			if key, ok := idKey(cloudGet(item, "id")); ok {
				byID[key] = k
			}
		}
	}
	return byID
}

func (rd *awsReader) segments(items []orderedjson.Value, words []wordgroup.Word) ([]*model.Cue, error) {
	byID := awsWordIDs(words)
	used := make([]bool, len(words))
	var cues []*model.Cue
	for i, seg := range items {
		c, err := rd.segment(i, seg, words, byID, used)
		if err != nil {
			return nil, err
		}
		if c != nil {
			cues = append(cues, c)
		}
	}
	return cues, nil
}

// segment makes the cue of audio segment i from the words that its item ids name. A word that an earlier segment, or
// the same segment, already named belongs to that one, so the cost of damaged input stays linear.
func (rd *awsReader) segment(i int, seg orderedjson.Value, words []wordgroup.Word, byID map[string]int, used []bool) (*model.Cue, error) {
	start, end, text, err := awsSegmentFields(seg, "results.audio_segments["+strconv.Itoa(i)+"]")
	if err != nil {
		return nil, cloudFail(rd.st, err, i, seg)
	}
	var segWords []wordgroup.Word
	var raw []orderedjson.Value
	for _, id := range cloudGet(seg, "items").Items() {
		key, ok := idKey(id)
		if k, found := byID[key]; ok && found && !used[k] {
			segWords = append(segWords, words[k])
			raw = append(raw, words[k].Source...)
			used[k] = true
		}
	}
	speaker, _ := wordgroup.Speaker(cloudGet(seg, "speaker_label"))
	data := seg.With("items", orderedjson.ArrayOf(raw...))
	return rd.cues.Cue(start, end, text, segWords, speaker, cloudData(data), i, seg)
}

func awsSegmentFields(seg orderedjson.Value, path string) (start, end time.Duration, text string, err error) {
	v, _ := seg.Get("start_time")
	if start, err = wordgroup.Seconds(v, path+".start_time", time.Second); err != nil {
		return 0, 0, "", err
	}
	v, _ = seg.Get("end_time")
	if end, err = wordgroup.Seconds(v, path+".end_time", time.Second); err != nil {
		return 0, 0, "", err
	}
	text, err = wordgroup.Text(seg, "transcript", path)
	return start, end, text, err
}

func awsIsPunctuation(item orderedjson.Value) bool {
	typ, _ := item.Get("type")
	s, _ := typ.AsString()
	return s == "punctuation"
}
