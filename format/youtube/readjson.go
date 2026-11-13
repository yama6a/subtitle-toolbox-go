package youtube

import (
	"fmt"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// lookups are the pens and the window positions of a json3 file, which events name by index.
type lookups struct {
	pens, positions []orderedjson.Value
}

func (rd *reader) readJSON(content []byte) (*model.Subtitle, error) {
	root, err := orderedjson.Decode(content)
	if err != nil {
		return nil, &model.ParseError{
			Format: model.YouTubeTimedText, Msg: "The content is not valid JSON: " + err.Error() + ".", Err: err,
		}
	}
	events, _ := root.Get("events")
	if events.Kind() != orderedjson.Array {
		return nil, rd.parseError(model.NoLine, `The JSON has no "events" list.`)
	}
	pens, _ := root.Get("pens")
	positions, _ := root.Get("wpWinPositions")
	lk := lookups{pens: pens.Items(), positions: positions.Items()}
	fileValue := orderedjson.Union(
		orderedjson.ObjectOf(orderedjson.Member{Key: "format", Value: orderedjson.StringValue(variantJSON3)}),
		root.Without("events"))
	file, _ := jsondata.ToData(fileValue).(*model.Data)

	var caps []caption
	for i, event := range events.Items() {
		if isWindowEvent(event) {
			appendList(file, "windows", jsondata.ToData(event))
			continue
		}
		c, perr := readEvent(event, i, lk)
		if perr != nil {
			if err := rd.st.Fail(perr, model.NoLine, i, []string{orderedjson.Block(event)}); err != nil {
				return nil, err
			}
			continue
		}
		caps = append(caps, c)
	}
	return assemble(rd.st, caps, file, rd.wordTimes)
}

// isWindowEvent reports whether event defines a window: an object without segs and with an id.
func isWindowEvent(event orderedjson.Value) bool {
	if event.Kind() != orderedjson.Object {
		return false
	}
	return !isSet(event, "segs") && isSet(event, "id")
}

// jsonTime reads the number under key of object in milliseconds. A missing or null value is 0 when optional.
func jsonTime(object orderedjson.Value, key, path string, optional bool) (time.Duration, *model.ParseError) {
	v, _ := object.Get(key)
	path += "." + key
	if v.Kind() == orderedjson.Null && optional {
		return 0, nil
	}
	if f, ok := v.AsFloat64(); !ok || f < 0 {
		return 0, &model.ParseError{Msg: "The field " + path + " must be a number."}
	}
	d, ok := v.Duration(time.Millisecond)
	if !ok {
		return 0, overLimit(path)
	}
	return d, nil
}

func overLimit(path string) *model.ParseError {
	_, err := parsing.BoundedField(1<<62, path)
	return asParseError(err)
}

func readEvent(event orderedjson.Value, i int, lk lookups) (caption, *model.ParseError) {
	path := "events[" + strconv.Itoa(i) + "]"
	start, perr := jsonTime(event, "tStartMs", path, false)
	if perr != nil {
		return caption{}, perr
	}
	dur, perr := jsonTime(event, "dDurationMs", path, true)
	if perr != nil {
		return caption{}, perr
	}
	end := start + dur
	if _, err := parsing.BoundedField(end, path+".dDurationMs"); err != nil {
		return caption{}, asParseError(err)
	}
	segs, extras, perr := readJSONSegments(event, path, start, lk)
	if perr != nil {
		return caption{}, perr
	}
	data, _ := jsondata.ToData(event.Without("segs")).(*model.Data)
	if hasMembers(extras) {
		data.Set("segments", extras)
	}
	source, _ := jsondata.ToData(event).(*model.Data)
	c := caption{
		timing:   timing{start: start, end: end, hasEnd: true},
		isAppend: !emptyValue(valueOf(event, "aAppend")),
		segments: segs,
		data:     data,
		source:   source,
	}
	if w := valueOf(event, "wWinId"); w.Kind() != orderedjson.Null {
		c.window, c.hasWindow = orderedjson.Block(w), true
	}
	if pos, ok := entry(lk.positions, valueOf(event, "wpWinPosId")); ok {
		c.alignment = anchorAlignment(pos)
	}
	return c, nil
}

// anchorAlignment returns the alignment of the apPoint of a window position, bottom center for none.
func anchorAlignment(position orderedjson.Value) model.Alignment {
	anchor, ok := valueOf(position, "apPoint").AsInt64()
	if !ok || anchor < 0 || anchor >= int64(len(anchorAlignments)) {
		return model.BottomCenter
	}
	return anchorAlignments[anchor]
}

func valueOf(object orderedjson.Value, key string) orderedjson.Value {
	v, _ := object.Get(key)
	return v
}

func hasMembers(extras []any) bool {
	for _, e := range extras {
		if d, ok := e.(*model.Data); ok && d.Len() > 0 {
			return true
		}
	}
	return false
}

// entry returns the pen or window position that an integer id names, when it is an object or an array.
func entry(list []orderedjson.Value, id orderedjson.Value) (orderedjson.Value, bool) {
	i, ok := id.AsInt64()
	if !ok || i < 0 || i >= int64(len(list)) {
		return orderedjson.Value{}, false
	}
	e := list[i]
	switch e.Kind() {
	case orderedjson.Object:
		return e, true
	case orderedjson.Array:
		return orderedjson.ObjectOf(), true
	case orderedjson.Null, orderedjson.Bool, orderedjson.Number, orderedjson.String:
	}
	return orderedjson.Value{}, false
}

// emptyValue reports whether v is empty as PHP empty() says: null, false, 0, "", "0" and an empty list.
func emptyValue(v orderedjson.Value) bool {
	switch v.Kind() {
	case orderedjson.Null:
		return true
	case orderedjson.Bool:
		on, _ := v.AsBool()
		return !on
	case orderedjson.Number:
		f, ok := v.AsFloat64()
		return ok && f == 0
	case orderedjson.String:
		s, _ := v.AsString()
		return s == "" || s == "0"
	case orderedjson.Array:
		return len(v.Items()) == 0
	case orderedjson.Object:
		return len(v.Members()) == 0
	}
	return false
}

// jsonPenStyle returns the style of a json3 pen.
func jsonPenStyle(pen orderedjson.Value) style {
	st := style{
		bold:      !emptyValue(valueOf(pen, "bAttr")),
		italic:    !emptyValue(valueOf(pen, "iAttr")),
		underline: !emptyValue(valueOf(pen, "uAttr")),
	}
	if c, ok := valueOf(pen, "fcForeColor").AsInt64(); ok && c >= 0 && c <= 0xFFFFFF {
		st.color = fmt.Sprintf("#%06x", c)
	}
	return st
}

// readJSONSegments returns the segments of an event and the stored object of each.
func readJSONSegments(event orderedjson.Value, path string, start time.Duration, lk lookups) ([]segment, []any, *model.ParseError) {
	list := valueOf(event, "segs")
	var segs []segment
	var extras []any
	for j, item := range list.Items() {
		segPath := path + ".segs[" + strconv.Itoa(j) + "]"
		var seg segment
		if v := valueOf(item, "tOffsetMs"); v.Kind() != orderedjson.Null {
			d, perr := jsonTime(item, "tOffsetMs", segPath, false)
			if perr != nil {
				return nil, nil, perr
			}
			if _, err := parsing.BoundedField(start+d, segPath+".tOffsetMs"); err != nil {
				return nil, nil, asParseError(err)
			}
			seg.offset, seg.hasOffset = d, true
		}
		seg.text = stringOf(valueOf(item, "utf8"))
		penID := valueOf(item, "pPenId")
		if penID.Kind() == orderedjson.Null {
			penID = valueOf(event, "pPenId")
		}
		if pen, ok := entry(lk.pens, penID); ok {
			seg.style = jsonPenStyle(pen)
		}
		segs = append(segs, seg)
		extra := model.NewData()
		if item.Kind() == orderedjson.Object {
			extra, _ = jsondata.ToData(item).(*model.Data)
		}
		extras = append(extras, extra)
	}
	return segs, extras, nil
}
