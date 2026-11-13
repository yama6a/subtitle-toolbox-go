package transcript

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// ReadWhisper reads the JSON of the OpenAI transcription API, openai-whisper, faster-whisper, WhisperX and
// whisper.cpp from r. It makes one cue per segment. A field error names the path of the field, such as
// segments[3].start.
func ReadWhisper(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.Whisper, err)
	}
	st := parsing.NewState(model.Whisper, common)
	// The JSON decoder reads each bad byte as U+FFFD in both modes, so the UTF-8 check only warns.
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	root, err := decodeObject(content, model.Whisper)
	if err != nil {
		return nil, nil, err
	}
	rd := &whisperReader{
		st: st,
		cues: wordgroup.NewReader(st, wordgroup.Options{
			WordTimestamps: opts.WordTimestamps, SpeakerVoices: opts.SpeakerVoices,
		}, whisperKey),
	}
	s, err := rd.read(root)
	if err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func decodeObject(content []byte, f model.Format) (orderedjson.Value, error) {
	v, err := orderedjson.Decode(content)
	if err != nil {
		msg := "The content is not valid JSON: " + err.Error() + "."
		return v, &model.ParseError{Format: f, Msg: msg, Err: err}
	}
	if v.Kind() != orderedjson.Object {
		return v, &model.ParseError{Format: f, Msg: "The JSON root must be an object."}
	}
	return v, nil
}

type whisperReader struct {
	st   *parsing.State
	cues *wordgroup.Reader
}

func (rd *whisperReader) read(root orderedjson.Value) (*model.Subtitle, error) {
	var cues []*model.Cue
	var err error
	switch {
	case isList(root, "segments"):
		segments, _ := root.Get("segments")
		words, _ := root.Get("words")
		cues, err = rd.segments(segments.Items(), words.Items())
	case isList(root, "transcription"):
		items, _ := root.Get("transcription")
		cues, err = rd.transcription(items.Items())
	default:
		err = &model.ParseError{Format: model.Whisper, Msg: `The JSON has no "segments" or "transcription" list.`}
	}
	if err != nil {
		return nil, err
	}
	fileData := orderedjson.Union(
		orderedjson.ObjectOf(orderedjson.Member{Key: "format", Value: orderedjson.StringValue(shapeOf(root))}),
		root.Without("segments", "transcription", "words", "word_segments", "text"),
	)
	data, _ := jsondata.ToData(fileData).(*model.Data)
	return rd.cues.Transcript(cues, language(root), data)
}

// language returns the language of the file with its name turned into its code, as the OpenAI API names it.
func language(root orderedjson.Value) orderedjson.Value {
	v, ok := root.Get("language")
	if !ok || v.Kind() == orderedjson.Null {
		result, _ := root.Get("result")
		v, _ = result.Get("language")
	}
	name, ok := v.AsString()
	if !ok {
		return v
	}
	if code, ok := whisperLanguages[asciiLower(name)]; ok {
		return orderedjson.StringValue(code)
	}
	return v
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

func (rd *whisperReader) fail(perr *model.ParseError, block int, item orderedjson.Value) error {
	return rd.st.Fail(perr, model.NoLine, block, []string{orderedjson.Block(item)})
}

func fieldError(msg string) *model.ParseError { return &model.ParseError{Msg: msg} }

// number reads the time under key of object times unit. The field must be a finite number of 0 or more.
func number(object orderedjson.Value, key, path string, unit time.Duration) (time.Duration, *model.ParseError) {
	v, _ := object.Get(key)
	path += "." + key
	if _, ok := v.AsFloat64(); !ok {
		return 0, fieldError("The field " + path + " must be a number.")
	}
	d, state := timeOf(v, unit)
	switch state {
	case noTime:
		return 0, fieldError("The field " + path + " must be a time of 0 or more.")
	case overLimit:
		return 0, overLimitError(path)
	case validTime:
	}
	return d, nil
}

func overLimitError(path string) *model.ParseError {
	_, err := parsing.BoundedField(timecode.MaxTime, path)
	return asParseError(err)
}

func asParseError(err error) *model.ParseError {
	var perr *model.ParseError
	errors.As(err, &perr)
	return perr
}

// segments reads the segments of the openai-whisper, openai-api and whisperx shapes. top holds the words at the
// top level of the OpenAI API, which the segments take in order.
func (rd *whisperReader) segments(items, top []orderedjson.Value) ([]*model.Cue, error) {
	var cues []*model.Cue
	next := 0
	for i, seg := range items {
		cue, err := rd.segment(i, seg, top, &next, i == len(items)-1)
		if err != nil {
			return nil, err
		}
		if cue != nil {
			cues = append(cues, cue)
		}
	}
	return cues, nil
}

// segment reads segment i. It takes the top-level words from *next on whose middle is at most the segment end, or
// all of them when last is true. A segment that fails before that takes none, so the next segment gets its words.
func (rd *whisperReader) segment(i int, seg orderedjson.Value, top []orderedjson.Value, next *int, last bool) (*model.Cue, error) {
	path := "segments[" + strconv.Itoa(i) + "]"
	start, end, text, perr := segmentFields(seg, path, time.Second)
	if perr != nil {
		return nil, rd.fail(perr, i, seg)
	}
	own, _ := seg.Get("words")
	words := slices.Clone(own.Items())
	ownCount, firstTop := len(words), *next
	if len(top) > 0 {
		for *next < len(top) && (last || middle(top[*next]) <= float64(end.Milliseconds())/1000) {
			words = append(words, top[*next])
			*next++
		}
		seg = seg.With("words", orderedjson.ArrayOf(words...))
	}
	timed, perr := segmentWords(words, func(j int) string {
		if j < ownCount {
			return path + ".words[" + strconv.Itoa(j) + "]"
		}
		return "words[" + strconv.Itoa(firstTop+j-ownCount) + "]"
	})
	if perr != nil {
		return nil, rd.fail(perr, i, seg)
	}
	return rd.cue(start, end, text, timed, seg, i)
}

func segmentFields(seg orderedjson.Value, path string, unit time.Duration) (start, end time.Duration, text string, perr *model.ParseError) {
	if start, perr = number(seg, "start", path, unit); perr != nil {
		return 0, 0, "", perr
	}
	if end, perr = number(seg, "end", path, unit); perr != nil {
		return 0, 0, "", perr
	}
	text, err := wordgroup.Text(seg, "text", path)
	return start, end, text, asParseError(err)
}

func (rd *whisperReader) cue(start, end time.Duration, text string, words []wordgroup.Word, item orderedjson.Value, block int) (*model.Cue, error) {
	v, _ := item.Get("speaker")
	var speaker string
	if name, ok := v.AsString(); ok {
		speaker = name
	}
	data, _ := jsondata.ToData(item).(*model.Data)
	return rd.cues.Cue(start, end, text, words, speaker, data, block, item)
}

// middle returns the middle of the word times in seconds, or minus infinity for a word without times.
func middle(word orderedjson.Value) float64 {
	start, _ := word.Get("start")
	end, ok := word.Get("end")
	if !ok || end.Kind() == orderedjson.Null {
		end = start
	}
	s, ok1 := numeric(start)
	e, ok2 := numeric(end)
	if !ok1 || !ok2 {
		return math.Inf(-1)
	}
	return (s + e) / 2
}

// numeric reads a number or a numeric string as PHP is_numeric does. A value out of range is an infinity.
func numeric(v orderedjson.Value) (float64, bool) {
	text, ok := v.Text()
	if !ok {
		s, isString := v.AsString()
		if !isString {
			return 0, false
		}
		if text, ok = orderedjson.NumericString(s); !ok {
			return 0, false
		}
	}
	f, _ := strconv.ParseFloat(text, 64)
	return f, true
}

// segmentWords reads the words of a segment. A word with a start of 100000 hours or more fails the segment.
// A word end of 100000 hours or more counts as no end.
func segmentWords(items []orderedjson.Value, path func(j int) string) ([]wordgroup.Word, *model.ParseError) {
	words := make([]wordgroup.Word, len(items))
	for j, item := range items {
		w := wordgroup.Word{Text: wordText(item), Confidence: wordConfidence(item)}
		v, _ := item.Get("start")
		switch d, state := timeOf(v, time.Second); state {
		case validTime:
			w.Start, w.Timed = d, true
		case overLimit:
			return nil, overLimitError(path(j) + ".start")
		case noTime:
		}
		v, _ = item.Get("end")
		if d, state := timeOf(v, time.Second); state == validTime {
			w.End, w.HasEnd = d, true
		}
		words[j] = w
	}
	return words, nil
}

func (rd *whisperReader) transcription(items []orderedjson.Value) ([]*model.Cue, error) {
	var cues []*model.Cue
	for i, item := range items {
		path := "transcription[" + strconv.Itoa(i) + "]"
		offsets, _ := item.Get("offsets")
		start, perr := number(offsets, "from", path+".offsets", time.Millisecond)
		var end time.Duration
		if perr == nil {
			end, perr = number(offsets, "to", path+".offsets", time.Millisecond)
		}
		var text string
		if perr == nil {
			var err error
			text, err = wordgroup.Text(item, "text", path)
			perr = asParseError(err)
		}
		var words []wordgroup.Word
		if perr == nil {
			tokens, _ := item.Get("tokens")
			words, perr = tokenTimes(tokenWords(tokens.Items()), path)
		}
		if perr != nil {
			if err := rd.fail(perr, i, item); err != nil {
				return nil, err
			}
			continue
		}
		cue, err := rd.cue(start, end, text, words, item, i)
		if err != nil {
			return nil, err
		}
		if cue != nil {
			cues = append(cues, cue)
		}
	}
	return cues, nil
}

// tokenTimes gives each word of whisper.cpp tokens the start of its first token and the end of its last token.
func tokenTimes(tokens []tokenWord, path string) ([]wordgroup.Word, *model.ParseError) {
	words := make([]wordgroup.Word, len(tokens))
	for i, t := range tokens {
		w := wordgroup.Word{Text: t.text}
		switch d, state := timeOf(offset(t.first, "from"), time.Millisecond); state {
		case validTime:
			w.Start, w.Timed = d, true
		case overLimit:
			return nil, overLimitError(path + ".tokens[" + strconv.Itoa(t.firstIndex) + "].offsets.from")
		case noTime:
		}
		if d, state := timeOf(offset(t.last, "to"), time.Millisecond); state == validTime {
			w.End, w.HasEnd = d, true
		}
		words[i] = w
	}
	return words, nil
}
