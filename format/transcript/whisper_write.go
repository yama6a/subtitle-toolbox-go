package transcript

import (
	"errors"
	"io"
	"math"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// WriteWhisper writes s to w as Whisper JSON in the shape of the tool that wrote the source, one segment per cue.
// The format field of the file format data names the shape. Cues from another format give the openai-whisper shape.
// A value that a cue did not change stays as the reader stored it. A cue without a speaker keeps the stored speaker.
func WriteWhisper(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.Whisper, common)
	if err != nil {
		return err
	}
	doc, err := whisperDocument(s)
	if err != nil {
		return err
	}
	body, err := encodeDocument(model.Whisper, "Whisper JSON", doc, false)
	if err != nil {
		return err
	}
	return writing.Write(w, model.Whisper, body, common, false)
}

// encodeDocument returns the JSON of doc, or an *UnwritableError that names the format as name.
func encodeDocument(f model.Format, name string, doc orderedjson.Value, pretty bool) ([]byte, error) {
	body, err := orderedjson.Encode(doc, pretty)
	if err == nil {
		return body, nil
	}
	reason := name + " cannot hold the subtitle: Inf and NaN cannot be JSON encoded."
	var cause error
	if errors.Is(err, orderedjson.ErrInvalidUTF8) {
		reason = name + " cannot hold the subtitle: Malformed UTF-8 characters, possibly incorrectly encoded. " +
			"Read a file in another encoding with its source encoding."
		cause = model.ErrInvalidUTF8
	}
	return nil, &model.UnwritableError{Format: f, CueIndex: -1, Reason: reason, Err: cause}
}

func member(key string, v orderedjson.Value) orderedjson.Member {
	return orderedjson.Member{Key: key, Value: v}
}

func whisperDocument(s *model.Subtitle) (orderedjson.Value, error) {
	var file orderedjson.Value
	if d, ok := s.FormatData(whisperKey); ok {
		var err error
		if file, err = jsondata.FromData(d); err != nil {
			return file, &model.UnwritableError{Format: model.Whisper, CueIndex: -1, Reason: "The Whisper format data is no JSON.", Err: err}
		}
	}
	shape := writtenShape(file)
	rest := file.Without("format")
	if lang, ok := s.Metadata(model.MetadataLanguage); ok && shape != shapeWhisperCPP {
		if v, _ := rest.Get("language"); v.Kind() == orderedjson.Null {
			rest = rest.With("language", orderedjson.StringValue(lang))
		}
	}

	ww := &whisperWriter{shape: shape}
	var segments []orderedjson.Value
	var text strings.Builder
	for _, c := range s.All() {
		seg := ww.entry(&c)
		t, _ := seg.Get("text")
		str, _ := t.AsString()
		text.WriteString(str)
		segments = append(segments, seg)
		ww.previous = seg
	}
	return ww.document(rest, orderedjson.ArrayOf(segments...), text.String()), nil
}

// writtenShape returns the shape that the format field of the file data names, else openai-whisper.
func writtenShape(file orderedjson.Value) string {
	v, _ := file.Get("format")
	if name, ok := v.AsString(); ok && (name == shapeOpenAIAPI || name == shapeWhisperX || name == shapeWhisperCPP) {
		return name
	}
	return shapeOpenAIWhisper
}

// entry returns the segment or the transcription entry of a cue.
func (ww *whisperWriter) entry(c *model.Cue) orderedjson.Value {
	switch ww.shape {
	case shapeWhisperCPP:
		return ww.cppEntry(c)
	case shapeOpenAIAPI:
		return ww.segment(c).Without("words")
	}
	return ww.segment(c)
}

// document returns the members of the file in the order of the shape, then the stored members of rest.
func (ww *whisperWriter) document(rest, segments orderedjson.Value, text string) orderedjson.Value {
	object := orderedjson.ObjectOf
	switch ww.shape {
	case shapeOpenAIAPI:
		doc := orderedjson.Union(rest, object(member("text", orderedjson.StringValue(strings.Trim(text, markupcore.PHPTrimSet)))))
		if len(ww.words) > 0 {
			doc = orderedjson.Union(doc, object(member("words", orderedjson.ArrayOf(ww.words...))))
		}
		return orderedjson.Union(doc, object(member("segments", segments)))
	case shapeWhisperX:
		return orderedjson.Union(object(member("segments", segments), member("word_segments", orderedjson.ArrayOf(ww.words...))), rest)
	case shapeWhisperCPP:
		return orderedjson.Union(rest, object(member("transcription", segments)))
	}
	return orderedjson.Union(object(member("text", orderedjson.StringValue(text)), member("segments", segments)), rest)
}

type whisperWriter struct {
	shape string
	// words holds the words of all segments written so far.
	words []orderedjson.Value
	// previous is the segment written before the current one.
	previous orderedjson.Value
}

// sameTime reports whether the member key of stored is a number that rounds to t.
func sameTime(stored orderedjson.Value, key string, unit, t time.Duration) bool {
	v, _ := stored.Get(key)
	return v.Kind() == orderedjson.Number && wordgroup.SameTime(v, unit, t)
}

func (ww *whisperWriter) segment(c *model.Cue) orderedjson.Value {
	stored := storedObject(c, whisperKey)
	sameStart := sameTime(stored, "start", time.Second, c.Start)
	sameEnd := sameTime(stored, "end", time.Second, c.End)
	storedText, _ := stored.Get("text")
	sameText := wordgroup.SameText(c, storedText)
	speaker := wordgroup.CueSpeaker(c)

	fields := orderedjson.ObjectOf(
		member("start", timeValue(stored, "start", sameStart, c.Start)),
		member("end", timeValue(stored, "end", sameEnd, c.End)),
		member("text", cueTextValue(c, storedText, sameText, segmentSpace(storedText))),
	)
	seg := orderedjson.Replace(ww.newID(stored), stored, fields)
	if !sameText {
		seg = seg.Without("tokens")
	}

	storedWords, _ := stored.Get("words")
	words, ok := ww.wordList(c, storedWords.Items(), sameStart && sameEnd, sameText, speaker)
	newSpeaker := changedSpeaker(stored, speaker)
	switch {
	case ok:
		if newSpeaker {
			words = withSpeaker(words, speaker)
		}
		seg = seg.With("words", orderedjson.ArrayOf(words...))
		ww.words = append(ww.words, words...)
	case storedWords.Kind() == orderedjson.Array:
		seg = seg.Without("words")
	}
	if newSpeaker {
		seg = seg.With("speaker", orderedjson.StringValue(speaker))
	}
	return seg
}

// newID returns an object with the id of a segment that has none. A segment gets the id of the segment before it plus 1
// when that is an integer, else 0. A WhisperX segment gets no id.
func (ww *whisperWriter) newID(stored orderedjson.Value) orderedjson.Value {
	if id, _ := stored.Get("id"); id.Kind() != orderedjson.Null || ww.shape == shapeWhisperX {
		return orderedjson.ObjectOf()
	}
	next := orderedjson.Int(0)
	if n, ok := ww.previousID(); ok {
		// An id past the int64 range becomes a float, as PHP does.
		next = orderedjson.Int(n + 1)
		if n == math.MaxInt64 {
			next = orderedjson.Float(float64(n) + 1)
		}
	}
	return orderedjson.ObjectOf(member("id", next))
}

func (ww *whisperWriter) previousID() (int64, bool) {
	id, _ := ww.previous.Get("id")
	return id.AsInt64()
}

// changedSpeaker reports whether the cue has a speaker that differs from the stored speaker.
func changedSpeaker(stored orderedjson.Value, speaker string) bool {
	v, _ := stored.Get("speaker")
	name, ok := v.AsString()
	return speaker != "" && (!ok || name != speaker)
}

// timeValue returns the stored time when it is the same as t, else the seconds of t.
func timeValue(stored orderedjson.Value, key string, same bool, t time.Duration) orderedjson.Value {
	if v, _ := stored.Get(key); same {
		return v
	}
	return orderedjson.Seconds(max(0, t))
}

func cueTextValue(c *model.Cue, stored orderedjson.Value, same bool, space string) orderedjson.Value {
	if same {
		return stored
	}
	return orderedjson.StringValue(space + wordgroup.CueText(c))
}

// segmentSpace returns the space that goes before the text of a segment. The reader keeps the text as the tool
// wrote it, so a stored text without a leading space gets none.
func segmentSpace(stored orderedjson.Value) string {
	if s, ok := stored.AsString(); ok && !strings.HasPrefix(s, " ") {
		return ""
	}
	return " "
}

func sourceWord(item orderedjson.Value) wordgroup.SourceWord {
	w := wordgroup.SourceWord{Text: wordText(item), Confidence: wordConfidence(item)}
	if v, _ := item.Get("start"); v.Kind() == orderedjson.Number {
		if d, state := timeOf(v, time.Second); state == validTime {
			w.Start = &d
		}
	}
	if v, _ := item.Get("end"); v.Kind() == orderedjson.Number {
		if d, state := timeOf(v, time.Second); state == validTime {
			w.End = &d
		}
	}
	return w
}

// wordList returns the words to write for the cue, and false when the cue gets no words.
func (ww *whisperWriter) wordList(c *model.Cue, stored []orderedjson.Value, sameTimes, sameText bool, speaker string) ([]orderedjson.Value, bool) {
	source := make([]wordgroup.SourceWord, len(stored))
	for i, item := range stored {
		source[i] = sourceWord(item)
	}
	choices, ok := wordgroup.Reuse(c, source, sameTimes, sameText)
	if !ok {
		return nil, false
	}
	var words []orderedjson.Value
	for _, ch := range choices {
		if ch.New != nil {
			words = append(words, ww.newWord(*ch.New, speaker))
			continue
		}
		for _, i := range ch.Source {
			item := stored[i]
			if ch.Word != nil {
				item = withTypedValues(item, ch.Word)
			}
			words = append(words, item)
		}
	}
	return words, true
}

// withTypedValues gives a reused source word the End and the Confidence of its entry in Cue.Words, for each member
// that the source word has and that differs. A stored end that is no valid time stays as it is.
func withTypedValues(item orderedjson.Value, w *model.Word) orderedjson.Value {
	if v, _ := item.Get("end"); v.Kind() == orderedjson.Number {
		if _, state := timeOf(v, time.Second); state == validTime && !sameTime(item, "end", time.Second, w.End) {
			item = item.With("end", orderedjson.Seconds(w.End))
		}
	}
	if w.Confidence == nil {
		return item
	}
	key := "probability"
	if v, ok := item.Get(key); !ok || v.Kind() == orderedjson.Null {
		key = "score"
	}
	if v, ok := item.Get(key); ok {
		if f, isNumber := v.AsFloat64(); !isNumber || f != *w.Confidence {
			item = item.With(key, orderedjson.Float(*w.Confidence))
		}
	}
	return item
}

func (ww *whisperWriter) newWord(n wordgroup.NewWord, speaker string) orderedjson.Value {
	start, end := orderedjson.Seconds(n.Start), orderedjson.Seconds(n.End)
	conf := orderedjson.Float(n.Confidence)
	switch ww.shape {
	case shapeOpenAIAPI:
		return orderedjson.ObjectOf(member("word", orderedjson.StringValue(n.Text)), member("start", start), member("end", end))
	case shapeWhisperX:
		m := []orderedjson.Member{
			member("word", orderedjson.StringValue(n.Text)), member("start", start), member("end", end), member("score", conf),
		}
		if speaker != "" {
			m = append(m, member("speaker", orderedjson.StringValue(speaker)))
		}
		return orderedjson.ObjectOf(m...)
	}
	return orderedjson.ObjectOf(
		member("word", orderedjson.StringValue(" "+n.Text)), member("start", start), member("end", end), member("probability", conf),
	)
}

// withSpeaker gives each word that has a speaker the new speaker.
func withSpeaker(words []orderedjson.Value, speaker string) []orderedjson.Value {
	out := make([]orderedjson.Value, len(words))
	for i, w := range words {
		out[i] = w
		if v, _ := w.Get("speaker"); v.Kind() != orderedjson.Null {
			out[i] = w.With("speaker", orderedjson.StringValue(speaker))
		}
	}
	return out
}

// cppEntry returns the transcription entry of a cue. Its tokens stay when the cue has the times, the text and the
// words of the stored entry. Else the entry has no tokens, because the writer cannot make token ids.
func (ww *whisperWriter) cppEntry(c *model.Cue) orderedjson.Value {
	stored := storedObject(c, whisperKey)
	from, to := max(0, c.Start).Round(time.Millisecond).Milliseconds(), max(0, c.End).Round(time.Millisecond).Milliseconds()
	offsets := orderedjson.ObjectOf(member("from", orderedjson.Int(from)), member("to", orderedjson.Int(to)))
	storedOffsets, _ := stored.Get("offsets")
	sameTimes := orderedjson.Equal(storedOffsets, offsets)
	storedText, _ := stored.Get("text")
	sameText := wordgroup.SameText(c, storedText)

	timestamps := orderedjson.ObjectOf(
		member("from", orderedjson.StringValue(clock(from))), member("to", orderedjson.StringValue(clock(to))),
	)
	if v, _ := stored.Get("timestamps"); sameTimes && v.Kind() != orderedjson.Null {
		timestamps = v
	}
	entry := orderedjson.Replace(stored, orderedjson.ObjectOf(
		member("timestamps", timestamps),
		member("offsets", offsets),
		member("text", cueTextValue(c, storedText, sameText, " ")),
	))
	tokens, _ := stored.Get("tokens")
	words := tokenWords(tokens.Items())
	source := make([]wordgroup.SourceWord, 0, len(words))
	for _, t := range words {
		w := wordgroup.SourceWord{Text: t.text}
		if d, state := timeOf(offset(t.first, "from"), time.Millisecond); state == validTime {
			w.Start = &d
		}
		if d, state := timeOf(offset(t.last, "to"), time.Millisecond); state == validTime {
			w.End = &d
		}
		source = append(source, w)
	}
	if !sameTimes || !sameText || !wordgroup.SameWords(c, source) {
		entry = entry.Without("tokens")
	}
	if speaker := wordgroup.CueSpeaker(c); changedSpeaker(stored, speaker) {
		entry = entry.With("speaker", orderedjson.StringValue(speaker))
	}
	return entry
}

func clock(ms int64) string {
	return string(timecode.AppendClock(nil, time.Duration(ms)*time.Millisecond, 2, ','))
}
