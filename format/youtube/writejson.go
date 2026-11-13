package youtube

import (
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// jsonLists holds the pens and the window positions of the document by index. New ones go to the end.
type jsonLists struct {
	pens, positions []orderedjson.Value
	penStyles       []style
	firstPen        map[style]int
	anchors         []int
	firstPosition   map[model.Alignment]int
}

func newJSONLists(pens, positions []orderedjson.Value) *jsonLists {
	l := &jsonLists{pens: pens, positions: positions, firstPen: map[style]int{}, firstPosition: map[model.Alignment]int{}}
	for i, p := range pens {
		st := jsonPenStyle(p)
		l.penStyles = append(l.penStyles, st)
		if _, ok := l.firstPen[st]; !ok {
			l.firstPen[st] = i
		}
	}
	for i, p := range positions {
		l.anchors = append(l.anchors, 7)
		if anchor, ok := valueOf(p, "apPoint").AsInt64(); ok && anchor >= 0 && anchor < 9 {
			l.anchors[i] = int(anchor)
		}
		a := anchorAlignments[l.anchors[i]]
		if _, ok := l.firstPosition[a]; !ok {
			l.firstPosition[a] = i
		}
	}
	return l
}

// list returns the stored array under name, else an array with one empty object.
func list(file orderedjson.Value, name string) []orderedjson.Value {
	v, _ := file.Get(name)
	if v.Kind() != orderedjson.Array {
		return []orderedjson.Value{orderedjson.ObjectOf()}
	}
	out := make([]orderedjson.Value, len(v.Items()))
	for i, item := range v.Items() {
		if item.Kind() == orderedjson.Array && len(item.Items()) == 0 {
			item = orderedjson.ObjectOf()
		}
		out[i] = item
	}
	return out
}

func intMember(object orderedjson.Value, name string) (int, bool) {
	i, ok := valueOf(object, name).AsInt64()
	return int(i), ok
}

// penOf returns the pen of a style: the preferred pen when it has the style, else the first pen with the style, else
// a new pen. needed is false for a plain style.
func (l *jsonLists) penOf(st style, preferred int, hasPreferred bool) (id int, needed bool) {
	if hasPreferred && preferred >= 0 && preferred < len(l.penStyles) && l.penStyles[preferred] == st {
		return preferred, true
	}
	if st == (style{}) {
		return 0, false
	}
	if i, ok := l.firstPen[st]; ok {
		return i, true
	}
	id = len(l.pens)
	l.pens = append(l.pens, newPen(st))
	l.penStyles = append(l.penStyles, st)
	l.firstPen[st] = id
	return id, true
}

// newPen returns the pen object of a style, with only the parts that the style has.
func newPen(st style) orderedjson.Value {
	var members []orderedjson.Member
	for _, f := range []struct {
		on   bool
		name string
	}{{st.bold, "bAttr"}, {st.italic, "iAttr"}, {st.underline, "uAttr"}} {
		if f.on {
			members = append(members, orderedjson.Member{Key: f.name, Value: orderedjson.Int(1)})
		}
	}
	if st.color != "" {
		var c int64
		for _, d := range st.color[1:] {
			c = c*16 + int64(strings.IndexRune("0123456789abcdef", d))
		}
		members = append(members, orderedjson.Member{Key: "fcForeColor", Value: orderedjson.Int(c)})
	}
	return orderedjson.ObjectOf(members...)
}

// position sets wpWinPosId of event to a window position with the alignment of the cue, as srv3 does.
func (l *jsonLists) position(event orderedjson.Value, a model.Alignment) orderedjson.Value {
	current, hasCurrent := intMember(event, "wpWinPosId")
	cur := model.BottomCenter
	if hasCurrent && current >= 0 && current < len(l.anchors) {
		cur = anchorAlignments[l.anchors[current]]
	}
	switch {
	case hasCurrent && cur == a:
		return event
	case a == model.BottomCenter:
		return event.Without("wpWinPosId")
	}
	id, ok := l.firstPosition[a]
	if !ok {
		anchor := anchorOf(a)
		id = len(l.positions)
		l.positions = append(l.positions, orderedjson.ObjectOf(
			orderedjson.Member{Key: "apPoint", Value: orderedjson.Int(int64(anchor))},
			orderedjson.Member{Key: "ahHorPos", Value: orderedjson.Int(int64(anchor % 3 * 50))},
			orderedjson.Member{Key: "avVerPos", Value: orderedjson.Int(int64(anchor / 3 * 50))},
		))
		l.anchors = append(l.anchors, anchor)
		l.firstPosition[a] = id
	}
	return event.With("wpWinPosId", orderedjson.Int(int64(id)))
}

// json3Document returns the document: the pens, window styles and window positions, then the events. The window
// events come first, and each cue has its append events before it.
func json3Document(cues []model.Cue, stored []orderedjson.Value, file orderedjson.Value) orderedjson.Value {
	lists := newJSONLists(list(file, "pens"), list(file, "wpWinPositions"))
	windows, _ := file.Get("windows")
	events := append([]orderedjson.Value(nil), windows.Items()...)
	keys := timeKeys{start: "tStartMs", duration: "dDurationMs", window: "wWinId", unit: time.Millisecond}
	for i, t := range keys.captions(cues, stored) {
		events = append(events, t.appends...)
		events = append(events, lists.event(cues[i], t.fields))
	}
	trailing, _ := file.Get("appends")
	events = append(events, trailing.Items()...)

	object := func(items []orderedjson.Value) orderedjson.Value { return orderedjson.ArrayOf(items...) }
	return orderedjson.Replace(
		orderedjson.ObjectOf(orderedjson.Member{Key: "wireMagic", Value: orderedjson.StringValue("pb3")}),
		file.Without("format", "windows", "appends"),
		orderedjson.ObjectOf(
			orderedjson.Member{Key: "pens", Value: object(lists.pens)},
			orderedjson.Member{Key: "wsWinStyles", Value: object(list(file, "wsWinStyles"))},
			orderedjson.Member{Key: "wpWinPositions", Value: object(lists.positions)},
		),
		orderedjson.ObjectOf(orderedjson.Member{Key: "events", Value: object(events)}),
	)
}

// storedSegments reads the stored segments of an event.
func (l *jsonLists) storedSegments(fields orderedjson.Value) []stored {
	eventPen, hasEventPen := intMember(fields, "pPenId")
	segs, _ := fields.Get("segments")
	source := make([]stored, 0, len(segs.Items()))
	for _, item := range segs.Items() {
		if item.Kind() != orderedjson.Object {
			item = orderedjson.ObjectOf()
		}
		s := stored{fields: item}
		s.text, _ = valueOf(item, "utf8").AsString()
		if v := valueOf(item, "tOffsetMs"); v.Kind() == orderedjson.Number {
			s.offset, s.hasOffset = v.Duration(time.Millisecond)
		}
		pen, has := eventPen, hasEventPen
		if v := valueOf(item, "pPenId"); v.Kind() != orderedjson.Null {
			pen, has = intMember(item, "pPenId")
		}
		if has && pen >= 0 && pen < len(l.penStyles) {
			s.style = l.penStyles[pen]
		}
		source = append(source, s)
	}
	return source
}

// event returns the event of a cue.
func (l *jsonLists) event(c model.Cue, fields orderedjson.Value) orderedjson.Value {
	segments, unchanged := segmentsOf(c, l.storedSegments(fields))
	event := l.position(fields.Without("segments", "appends"), c.Position.Alignment)
	out := make([]orderedjson.Value, len(segments))
	if unchanged {
		for i, s := range segments {
			out[i] = s.fields
		}
	} else {
		event = event.Without("pPenId")
		for i, s := range segments {
			out[i] = l.newSegment(s)
		}
	}
	return orderedjson.Union(event, orderedjson.ObjectOf(orderedjson.Member{Key: "segs", Value: orderedjson.ArrayOf(out...)}))
}

// newSegment writes a segment of a changed event: the text, the offset, the pen and the other stored fields.
func (l *jsonLists) newSegment(s stored) orderedjson.Value {
	preferred, hasPreferred := intMember(s.fields, "pPenId")
	pen, needed := l.penOf(s.style, preferred, hasPreferred)
	members := []orderedjson.Member{{Key: "utf8", Value: orderedjson.StringValue(s.text)}}
	if ms := s.offset.Milliseconds(); ms > 0 || isSet(s.fields, "tOffsetMs") {
		members = append(members, orderedjson.Member{Key: "tOffsetMs", Value: orderedjson.Int(ms)})
	}
	if needed {
		members = append(members, orderedjson.Member{Key: "pPenId", Value: orderedjson.Int(int64(pen))})
	}
	return orderedjson.Union(orderedjson.ObjectOf(members...), s.fields.Without("utf8", "tOffsetMs", "pPenId"))
}
