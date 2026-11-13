package transcript

import (
	"math"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// The helpers below are the same for the formats of the cloud speech-to-text services: Amazon Transcribe,
// Deepgram, AssemblyAI and Google Speech-to-Text.

func cloudData(v orderedjson.Value) *model.Data {
	d, _ := jsondata.ToData(v).(*model.Data)
	return d
}

func cloudGet(v orderedjson.Value, key string) orderedjson.Value {
	x, _ := v.Get(key)
	return x
}

// cloudConfidence returns a number from 0 to 1, else nil.
func cloudConfidence(v orderedjson.Value) *float64 {
	f, ok := v.AsFloat64()
	if !ok || math.IsNaN(f) || f < 0 || f > 1 {
		return nil
	}
	return &f
}

// cloudFileData returns the file format data under key as an object, or an empty object.
func cloudFileData(s *model.Subtitle, f model.Format) (orderedjson.Value, error) {
	d, ok := s.FormatData(string(f))
	if !ok {
		return orderedjson.ObjectOf(), nil
	}
	v, err := jsondata.FromData(d)
	if err != nil {
		return v, &model.UnwritableError{Format: f, CueIndex: -1, Reason: "The " + string(f) + " format data is no JSON.", Err: err}
	}
	if v.Kind() != orderedjson.Object {
		return orderedjson.ObjectOf(), nil
	}
	return v, nil
}

// cloudNumber returns v when it is a JSON number, else null.
func cloudNumber(v orderedjson.Value) orderedjson.Value {
	if v.Kind() == orderedjson.Number {
		return v
	}
	return orderedjson.Value{}
}

// cloudWithSpeaker sets the member speaker of each word that is an object.
func cloudWithSpeaker(words []orderedjson.Value, speaker orderedjson.Value) []orderedjson.Value {
	for i, w := range words {
		if w.Kind() == orderedjson.Object {
			words[i] = w.With("speaker", speaker)
		}
	}
	return words
}

// cloudSources returns the stored start, end and text of a cue. A cue without a stored text is grouped, so the values
// come from its stored words. wordText returns the trimmed text of a stored word.
func cloudSources(data orderedjson.Value, textKey string, stored []orderedjson.Value, grouped bool,
	wordText func(orderedjson.Value) string,
) (start, end, text orderedjson.Value) {
	if !grouped {
		return cloudGet(data, "start"), cloudGet(data, "end"), cloudGet(data, textKey)
	}
	if len(stored) > 0 {
		start, end = cloudGet(stored[0], "start"), cloudGet(stored[len(stored)-1], "end")
	}
	texts := make([]string, len(stored))
	for i, w := range stored {
		texts[i] = wordText(w)
	}
	return start, end, orderedjson.StringValue(strings.Join(texts, " "))
}

// cloudTyped gives a reused word the End and the Confidence of its entry in Cue.Words, for each member that the word has
// and that differs. unit is the unit of the stored end, and end writes a duration in that unit.
func cloudTyped(item orderedjson.Value, w *model.Word, unit time.Duration, end func(time.Duration) orderedjson.Value) orderedjson.Value {
	if item.Kind() != orderedjson.Object {
		return item
	}
	if v, ok := item.Get("end"); ok && !wordgroup.SameTime(cloudNumber(v), unit, w.End) {
		item = item.With("end", end(w.End))
	}
	if v, ok := item.Get("confidence"); ok && w.Confidence != nil {
		if f, isNumber := v.AsFloat64(); !isNumber || f != *w.Confidence {
			item = item.With("confidence", orderedjson.Float(*w.Confidence))
		}
	}
	return item
}
