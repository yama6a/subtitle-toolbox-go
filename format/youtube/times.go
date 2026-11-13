package youtube

import (
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// timeKeys names the time fields of a variant. unit is the length of the stored value 1: a second for srv1 and a
// millisecond for the others. asString is true for the XML variants, which write times as attribute text.
type timeKeys struct {
	start, duration, window string
	unit                    time.Duration
	asString                bool
	// zeroWhenMissing is true for json3, where a reader reads an event without dDurationMs as 0 s.
	zeroWhenMissing bool
}

// timedCaption is the stored fields of a cue with the start and the duration set, and the append captions before it.
type timedCaption struct {
	fields  orderedjson.Value
	appends []orderedjson.Value
}

// captions writes the start and the duration of each caption. An unedited time keeps its source value. A duration
// that the source lacked stays absent when a reader reads the same cue end from it again.
func (k timeKeys) captions(cues []model.Cue, stored []orderedjson.Value) []timedCaption {
	ts := make([]timing, len(cues))
	sources := make([]time.Duration, len(cues))
	hasSource := make([]bool, len(cues))
	for i, c := range cues {
		sources[i], hasSource[i] = k.seconds(stored[i], k.start)
		ts[i] = k.timing(c, stored[i], hasSource[i])
	}
	ends := endsOf(ts, writerLastCueDuration)

	out := make([]timedCaption, len(cues))
	for i, c := range cues {
		data := stored[i]
		start := ts[i].start
		end := max(start, c.End)
		fields := orderedjson.Union(data, orderedjson.ObjectOf(
			orderedjson.Member{Key: k.start}, orderedjson.Member{Key: k.duration}))
		if v, _ := data.Get(k.start); !wordgroup.SameTime(v, k.unit, start) {
			fields = fields.With(k.start, k.value(start))
		}
		if v, _ := data.Get(k.duration); !hasSource[i] || ends[i] != end {
			fields = fields.With(k.duration, k.value(end-start))
		} else if v.Kind() == orderedjson.Null {
			fields = fields.Without(k.duration)
		}
		var shift time.Duration
		if hasSource[i] {
			shift = start - sources[i]
		}
		out[i] = timedCaption{fields: fields, appends: k.appends(data, shift)}
	}
	return out
}

// timing returns the input of the end rule for a cue. A cue without a source start has the end of the cue. A cue
// with a source start but no source duration has no end. Else the end is the start plus the source duration.
func (k timeKeys) timing(c model.Cue, data orderedjson.Value, hasSource bool) timing {
	start := max(0, c.Start)
	t := timing{start: start, end: max(start, c.End), hasEnd: true}
	if hasSource {
		dur, ok := k.seconds(data, k.duration)
		if v, _ := data.Get(k.duration); !ok && k.zeroWhenMissing && v.Kind() == orderedjson.Null {
			dur, ok = 0, true
		}
		t.end, t.hasEnd = start+dur, ok
	}
	if k.window != "" {
		if w, _ := data.Get(k.window); w.Kind() != orderedjson.Null {
			t.window, t.hasWindow = orderedjson.Block(w), true
		}
	}
	return t
}

// appends returns the stored append captions of a cue, moved by shift.
func (k timeKeys) appends(data orderedjson.Value, shift time.Duration) []orderedjson.Value {
	list, _ := data.Get("appends")
	var out []orderedjson.Value
	for _, a := range list.Items() {
		if a.Kind() == orderedjson.Object || a.Kind() == orderedjson.Array {
			out = append(out, k.shift(a, shift))
		}
	}
	return out
}

// shift moves the start of an append caption by the shift of its cue.
func (k timeKeys) shift(a orderedjson.Value, shift time.Duration) orderedjson.Value {
	start, ok := k.seconds(a, k.start)
	if !ok || shift == 0 || a.Kind() != orderedjson.Object {
		return a
	}
	return a.With(k.start, k.value(max(0, start+shift)))
}

// seconds returns the time under name when it is a number or a numeric string.
func (k timeKeys) seconds(object orderedjson.Value, name string) (time.Duration, bool) {
	v, _ := object.Get(name)
	return v.Duration(k.unit)
}

// value writes d in the unit of the variant: "4.4" for srv1, "4400" for srv2 and srv3, 4400 for json3.
func (k timeKeys) value(d time.Duration) orderedjson.Value {
	ms := d.Milliseconds()
	switch {
	case !k.asString:
		return orderedjson.Int(ms)
	case k.unit == time.Millisecond:
		return orderedjson.StringValue(strconv.FormatInt(ms, 10))
	}
	text := strconv.FormatInt(ms/1000, 10) + "." + pad3(ms%1000)
	for text[len(text)-1] == '0' {
		text = text[:len(text)-1]
	}
	if text[len(text)-1] == '.' {
		text = text[:len(text)-1]
	}
	return orderedjson.StringValue(text)
}

func pad3(n int64) string {
	s := strconv.FormatInt(n, 10)
	return "000"[len(s):] + s
}
