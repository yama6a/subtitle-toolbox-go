package wordgroup

import (
	"math"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Seconds reads the time v times unit, rounded to whole milliseconds. v is a number or a numeric string.
// A missing field is the null Value. The error has no format, so State.Fail adds it.
func Seconds(v orderedjson.Value, path string, unit time.Duration) (time.Duration, error) {
	if !validTime(v) {
		return 0, &model.ParseError{Msg: "The field " + path + " must be a time."}
	}
	d, ok := v.Duration(unit)
	if !ok {
		return parsing.BoundedField(timecode.MaxTime, path)
	}
	return d, nil
}

func validTime(v orderedjson.Value) bool {
	var f float64
	var ok bool
	switch v.Kind() {
	case orderedjson.Number:
		f, ok = v.AsFloat64()
	case orderedjson.String:
		s, _ := v.AsString()
		if s, ok = orderedjson.NumericString(s); ok {
			var err error
			f, err = strconv.ParseFloat(s, 64)
			ok = err == nil
		}
	case orderedjson.Null, orderedjson.Bool, orderedjson.Array, orderedjson.Object:
	}
	return ok && !math.IsInf(f, 0) && !math.IsNaN(f) && f >= 0
}

// Text returns the string under key of object. A value that is no object has no key.
func Text(object orderedjson.Value, key, path string) (string, error) {
	v, _ := object.Get(key)
	s, ok := v.AsString()
	if !ok {
		return "", &model.ParseError{Msg: "The field " + path + "." + key + " must be a string."}
	}
	return s, nil
}

// Speaker returns a speaker that is a string or an integer. Anything else is no speaker.
func Speaker(v orderedjson.Value) (string, bool) {
	if s, ok := v.AsString(); ok {
		return s, true
	}
	if i, ok := v.AsInt64(); ok {
		return strconv.FormatInt(i, 10), true
	}
	return "", false
}

// Between returns the words from *index on whose middle lies between start and end, and moves *index past them.
// It first skips the words whose middle lies before start. The middle is a float64 of seconds, as in PHP, so a word
// on a boundary falls to the same side as in PHP.
func Between(words []Word, index *int, start, end time.Duration) []Word {
	middle := func(w Word) float64 { return (w.Start.Seconds() + w.end().Seconds()) / 2 }
	for *index < len(words) && middle(words[*index]) < start.Seconds() {
		*index++
	}
	from := *index
	for *index < len(words) && middle(words[*index]) <= end.Seconds() {
		*index++
	}
	return words[from:*index]
}

// WithResults returns the members of data without "results", then the members of results without readKeys under
// "results" when any are left.
func WithResults(data, results orderedjson.Value, readKeys ...string) orderedjson.Value {
	out := data.Without("results")
	if other := results.Without(readKeys...); len(other.Members()) > 0 {
		out = out.With("results", other)
	}
	return out
}
