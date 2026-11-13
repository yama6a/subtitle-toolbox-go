package youtube

import (
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// scalarText returns a string, number or bool value as PHP casts it to a string, and "" for another value.
func scalarText(v orderedjson.Value) string {
	switch v.Kind() {
	case orderedjson.String:
		s, _ := v.AsString()
		return s
	case orderedjson.Number:
		return numberText(v)
	case orderedjson.Bool:
		if on, _ := v.AsBool(); on {
			return "1"
		}
	case orderedjson.Null, orderedjson.Array, orderedjson.Object:
	}
	return ""
}

func isScalar(v orderedjson.Value) bool {
	k := v.Kind()
	return k == orderedjson.String || k == orderedjson.Number || k == orderedjson.Bool
}

// leadingInt returns the integer at the start of s as PHP intval reads it, 0 for none.
func leadingInt(s string) int {
	s = strings.TrimLeft(s, asciiSpace)
	end := 0
	if end < len(s) && (s[end] == '-' || s[end] == '+') {
		end++
	}
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}

// objects returns the items of an array that are objects or arrays. An array counts as an object without members.
func objects(list orderedjson.Value) []orderedjson.Value {
	var out []orderedjson.Value
	for _, item := range list.Items() {
		switch item.Kind() {
		case orderedjson.Object:
			out = append(out, item)
		case orderedjson.Array:
			out = append(out, orderedjson.ObjectOf())
		case orderedjson.Null, orderedjson.Bool, orderedjson.Number, orderedjson.String:
		}
	}
	return out
}

// penTable holds the pens of the head by id. A later pen with the same id wins, and the first pen with a style
// is the first id in file order that has the style.
type penTable struct {
	elements []orderedjson.Value
	styles   map[string]style
	first    map[style]string
	maxID    int
}

func newPenTable(elements []orderedjson.Value) *penTable {
	t := &penTable{elements: elements, styles: map[string]style{}, first: map[style]string{}}
	var order []string
	for _, e := range elements {
		id := t.idOf(e)
		if _, seen := t.styles[id]; !seen {
			order = append(order, id)
		}
		t.styles[id] = xmlPenStyle(func(name string) string { return stringMember(e, name) })
	}
	for _, id := range order {
		if _, ok := t.first[t.styles[id]]; !ok {
			t.first[t.styles[id]] = id
		}
		t.maxID = max(t.maxID, leadingInt(id))
	}
	return t
}

func (t *penTable) idOf(e orderedjson.Value) string {
	v, _ := e.Get("id")
	return scalarText(v)
}

// stringMember returns the member name when it is a string, else "".
func stringMember(object orderedjson.Value, name string) string {
	v, _ := object.Get(name)
	s, _ := v.AsString()
	return s
}

// styleOfPen returns the style of the pen with the id, plain for a missing pen.
func (t *penTable) styleOfPen(id string) style { return t.styles[id] }

// find returns the preferred pen when it has the style, else the first pen with the style. A plain style needs no
// pen. found is false when no pen has the style.
func (t *penTable) find(st style, preferred string, hasPreferred bool) (id string, needed, found bool) {
	if hasPreferred {
		if s, ok := t.styles[preferred]; ok && s == st {
			return preferred, true, true
		}
	}
	if st == (style{}) {
		return "", false, true
	}
	id, found = t.first[st]
	return id, true, found
}

// pen returns the pen of a style, and adds a pen when none has the style. needed is false for a plain style.
func (t *penTable) pen(st style, preferred string, hasPreferred bool) (id string, needed bool) {
	id, needed, found := t.find(st, preferred, hasPreferred)
	if found {
		return id, needed
	}
	t.maxID++
	id = strconv.Itoa(t.maxID)
	members := []orderedjson.Member{{Key: "id", Value: orderedjson.StringValue(id)}}
	for _, f := range []struct {
		on   bool
		name string
	}{{st.bold, "b"}, {st.italic, "i"}, {st.underline, "u"}} {
		if f.on {
			members = append(members, orderedjson.Member{Key: f.name, Value: orderedjson.StringValue("1")})
		}
	}
	if st.color != "" {
		members = append(members, orderedjson.Member{Key: "fc", Value: orderedjson.StringValue(strings.ToUpper(st.color))})
	}
	t.elements = append(t.elements, orderedjson.ObjectOf(members...))
	t.styles[id] = st
	if _, ok := t.first[st]; !ok {
		t.first[st] = id
	}
	return id, true
}

// positionTable holds the window positions of the head by id.
type positionTable struct {
	elements   []orderedjson.Value
	alignments map[string]model.Alignment
	first      map[model.Alignment]string
	maxID      int
}

func newPositionTable(elements []orderedjson.Value) *positionTable {
	t := &positionTable{elements: elements, alignments: map[string]model.Alignment{}, first: map[model.Alignment]string{}}
	var order []string
	for _, e := range elements {
		v, _ := e.Get("id")
		id := scalarText(v)
		if _, seen := t.alignments[id]; !seen {
			order = append(order, id)
		}
		t.alignments[id] = model.BottomCenter
		if ap, _ := e.Get("ap"); isNumeric(ap) {
			if a, valid := alignmentOf(truncated(scalarText(ap))); valid {
				t.alignments[id] = a
			}
		}
	}
	for _, id := range order {
		if _, ok := t.first[t.alignments[id]]; !ok {
			t.first[t.alignments[id]] = id
		}
		t.maxID = max(t.maxID, leadingInt(id))
	}
	return t
}

// truncated returns the integer part of a numeric text, as a PHP int cast does. A value out of range is -1.
func truncated(text string) int {
	f, _ := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if f < 0 || f >= 1<<30 {
		return -1
	}
	return int(f)
}

func isNumeric(v orderedjson.Value) bool {
	if v.Kind() == orderedjson.Number {
		return true
	}
	s, ok := v.AsString()
	if !ok {
		return false
	}
	_, ok = orderedjson.NumericString(s)
	return ok
}

// apply sets wp in fields to a window position with the alignment of the cue. A position that the cue keeps stays,
// bottom center needs none, and a new position is added when none has the alignment.
func (t *positionTable) apply(fields orderedjson.Value, a model.Alignment) orderedjson.Value {
	current, hasCurrent := "", false
	if v, _ := fields.Get("wp"); isScalar(v) {
		current, hasCurrent = scalarText(v), true
	}
	cur := model.BottomCenter
	if known, ok := t.alignments[current]; hasCurrent && ok {
		cur = known
	}
	switch {
	case hasCurrent && cur == a:
		return fields.With("wp", orderedjson.StringValue(current))
	case a == model.BottomCenter:
		return fields.Without("wp")
	}
	id, ok := t.first[a]
	if !ok {
		id = t.add(a)
	}
	return fields.With("wp", orderedjson.StringValue(id))
}

func (t *positionTable) add(a model.Alignment) string {
	anchor := anchorOf(a)
	t.maxID++
	id := strconv.Itoa(t.maxID)
	str := func(n int) orderedjson.Value { return orderedjson.StringValue(strconv.Itoa(n)) }
	t.elements = append(t.elements, orderedjson.ObjectOf(
		orderedjson.Member{Key: "id", Value: orderedjson.StringValue(id)},
		orderedjson.Member{Key: "ap", Value: str(anchor)},
		orderedjson.Member{Key: "ah", Value: str(anchor % 3 * 50)},
		orderedjson.Member{Key: "av", Value: str(anchor / 3 * 50)},
	))
	t.alignments[id] = a
	if _, ok := t.first[a]; !ok {
		t.first[a] = id
	}
	return id
}

// srv3 writes the head with the pens, window styles and window positions, then the body with the windows, the
// paragraphs and the append paragraphs.
func (xw *xmlWriter) srv3(cues []model.Cue, stored []orderedjson.Value, file orderedjson.Value) string {
	list := func(name string) []orderedjson.Value {
		v, _ := file.Get(name)
		return objects(v)
	}
	pens, positions, ws := newPenTable(list("pen")), newPositionTable(list("wp")), list("ws")
	var body strings.Builder
	windows, _ := file.Get("w")
	for _, w := range windows.Items() {
		if w.Kind() != orderedjson.Object {
			w = orderedjson.ObjectOf()
		}
		body.WriteString("<w" + xw.attributes(w) + "/>\n")
	}
	keys := timeKeys{start: "t", duration: "d", window: "w", unit: time.Millisecond, asString: true}
	for i, t := range keys.captions(cues, stored) {
		for _, a := range t.appends {
			body.WriteString(xw.appendParagraph(a))
		}
		body.WriteString(xw.paragraph(cues[i], t.fields, pens, positions) + "\n")
	}
	trailing, _ := file.Get("appends")
	for _, a := range objects(trailing) {
		body.WriteString(xw.appendParagraph(a))
	}

	var head strings.Builder
	for _, g := range []struct {
		name     string
		elements []orderedjson.Value
	}{{"pen", pens.elements}, {"ws", ws}, {"wp", positions.elements}} {
		for _, e := range g.elements {
			head.WriteString("<" + g.name + xw.attributes(e) + "/>\n")
		}
	}
	out := xmlDeclaration + "<timedtext format=\"3\">\n"
	if head.Len() > 0 {
		out += "<head>\n" + head.String() + "</head>\n"
	}
	return out + "<body>\n" + body.String() + "</body>\n</timedtext>\n"
}

func (xw *xmlWriter) appendParagraph(a orderedjson.Value) string {
	text := "\n"
	if v, _ := a.Get("text"); v.Kind() == orderedjson.String {
		text, _ = v.AsString()
	}
	return "<p" + xw.attributes(a, "text") + ">" + xw.escape(text) + "</p>\n"
}

// storedSegments reads the stored segments of a srv3 paragraph.
func storedSegments(fields orderedjson.Value, pens *penTable, paragraphPen string, hasParagraphPen bool) []stored {
	list, _ := fields.Get("segments")
	out := make([]stored, 0, len(list.Items()))
	for _, item := range list.Items() {
		if item.Kind() != orderedjson.Object {
			item = orderedjson.ObjectOf()
		}
		s := stored{fields: item}
		if v, _ := item.Get("text"); v.Kind() == orderedjson.String {
			raw, _ := v.AsString()
			s.text = markupcore.DecodeEntities(raw)
		}
		if v, _ := item.Get("t"); isNumeric(v) {
			s.offset, s.hasOffset = v.Duration(time.Millisecond)
		}
		pen, has := paragraphPen, hasParagraphPen
		if v, _ := item.Get("p"); v.Kind() != orderedjson.Null {
			pen, has = scalarText(v), true
		}
		if has {
			s.style = pens.styleOfPen(pen)
		}
		out = append(out, s)
	}
	return out
}

// paragraph writes the <p> element of a cue.
func (xw *xmlWriter) paragraph(c model.Cue, fields orderedjson.Value, pens *penTable, positions *positionTable) string {
	pv, _ := fields.Get("p")
	paragraphPen, hasPen := scalarText(pv), isScalar(pv)
	segments, unchanged := segmentsOf(c, storedSegments(fields, pens, paragraphPen, hasPen))
	attrs := positions.apply(fields.Without("text", "appends", "segments"), c.Position.Alignment)
	if unchanged {
		var b strings.Builder
		for _, s := range segments {
			b.WriteString(xw.span(s.fields))
		}
		return "<p" + xw.attributes(attrs) + ">" + b.String() + "</p>"
	}

	attrs = attrs.Without("p")
	if len(segments) == 1 && segments[0].offset == 0 && !isSet(segments[0].fields, "t") {
		pen, needed := pens.pen(segments[0].style, paragraphPen, hasPen)
		if needed {
			attrs = orderedjson.Union(attrs, orderedjson.ObjectOf(orderedjson.Member{Key: "p", Value: orderedjson.StringValue(pen)}))
		}
		return "<p" + xw.attributes(attrs) + ">" + xw.segmentText(segments[0]) + "</p>"
	}
	var b strings.Builder
	for _, s := range segments {
		b.WriteString(xw.newSpan(s, pens))
	}
	return "<p" + xw.attributes(attrs) + ">" + b.String() + "</p>"
}

// newSpan writes a segment of a changed paragraph: the offset, the pen and the other stored attributes. A segment
// without attributes is bare text.
func (xw *xmlWriter) newSpan(s stored, pens *penTable) string {
	spanPen, hasSpanPen := "", false
	if v, _ := s.fields.Get("p"); isScalar(v) {
		spanPen, hasSpanPen = scalarText(v), true
	}
	pen, needed := pens.pen(s.style, spanPen, hasSpanPen)
	members := []orderedjson.Member{}
	if ms := s.offset.Milliseconds(); ms > 0 || isSet(s.fields, "t") {
		members = append(members, orderedjson.Member{Key: "t", Value: orderedjson.StringValue(strconv.FormatInt(ms, 10))})
	}
	if needed {
		members = append(members, orderedjson.Member{Key: "p", Value: orderedjson.StringValue(pen)})
	}
	attrs := orderedjson.Union(orderedjson.ObjectOf(members...), s.fields.Without("text", "t", "p"))
	if len(attrs.Members()) == 0 {
		return xw.segmentText(s)
	}
	return "<s" + xw.attributes(attrs) + ">" + xw.segmentText(s) + "</s>"
}

func isSet(object orderedjson.Value, name string) bool {
	v, ok := object.Get(name)
	return ok && v.Kind() != orderedjson.Null
}

// span writes a stored segment: an <s> element when it has attributes, else its text.
func (xw *xmlWriter) span(fields orderedjson.Value) string {
	text := xw.escape(stringMember(fields, "text"))
	attrs := fields.Without("text")
	if len(attrs.Members()) == 0 {
		return text
	}
	return "<s" + xw.attributes(attrs) + ">" + text + "</s>"
}

// segmentText returns the escaped text of a segment. A text that the source wrote with escaped entities keeps them.
func (xw *xmlWriter) segmentText(s stored) string {
	if raw := stringMember(s.fields, "text"); markupcore.DecodeEntities(raw) == s.text {
		return xw.escape(raw)
	}
	return xw.escape(s.text)
}
