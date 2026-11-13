package youtube

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var (
	penColorRegexp = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	digitsRegexp   = regexp.MustCompile(`^[0-9]+$`)
)

// anchorAlignments is the alignment of each window anchor point. A point runs from 0, top left, to 8, bottom
// right, row by row.
//
//nolint:gochecknoglobals // a lookup table
var anchorAlignments = [9]model.Alignment{
	model.TopLeft, model.TopCenter, model.TopRight, model.MiddleLeft, model.MiddleCenter, model.MiddleRight,
	model.BottomLeft, model.BottomCenter, model.BottomRight,
}

// alignmentOf returns the alignment of an anchor point, false for a value outside 0 to 8.
func alignmentOf(anchor int) (model.Alignment, bool) {
	if anchor < 0 || anchor >= len(anchorAlignments) {
		return model.BottomCenter, false
	}
	return anchorAlignments[anchor], true
}

// anchorOf returns the anchor point of an alignment.
func anchorOf(a model.Alignment) int {
	for i, v := range anchorAlignments {
		if v == a {
			return i
		}
	}
	return 7
}

// appendList adds v to the list under name in d. A value that is no list stays.
func appendList(d *model.Data, name string, v any) {
	cur, ok := d.Get(name)
	if !ok {
		d.Set(name, []any{v})
	} else if list, isList := cur.([]any); isList {
		d.Set(name, append(list, v))
	}
}

// xmlPenStyle returns the style of a srv3 pen: the color as "#rrggbb" in lower case, b, i and u.
func xmlPenStyle(get func(name string) string) style {
	st := style{bold: get("b") == "1", italic: get("i") == "1", underline: get("u") == "1"}
	if c := get("fc"); penColorRegexp.MatchString(c) {
		st.color = strings.ToLower(c)
	}
	return st
}

func (rd *reader) readSrv3(root, body *xmlraw.Node) (*model.Subtitle, error) {
	file := model.NewData(model.Field{Key: "format", Value: variantSrv3})
	pens, anchors := readHead(root, file)
	var caps []caption
	for i, p := range elements(body) {
		switch xmlraw.QName(p.Name) {
		case "w":
			appendList(file, "w", attributes(p))
		case "p":
			c, perr := readParagraph(p, pens, anchors)
			if perr != nil {
				if err := rd.failElement(perr, p, i); err != nil {
					return nil, err
				}
				continue
			}
			caps = append(caps, c)
		}
	}
	return assemble(rd.st, caps, file, rd.wordTimes)
}

// readHead adds the elements of the head to file and returns the pen styles and the anchor points by id. A later
// element with the same id wins.
func readHead(root *xmlraw.Node, file *model.Data) (map[string]style, map[string]int) {
	pens := map[string]style{}
	anchors := map[string]int{}
	for _, section := range elements(root) {
		if xmlraw.QName(section.Name) != "head" {
			continue
		}
		for _, def := range elements(section) {
			attrs := attributes(def)
			name := xmlraw.QName(def.Name)
			appendList(file, name, attrs)
			id, _ := attrs.String("id")
			ap, _ := attr(def, "ap")
			switch {
			case name == "pen":
				pens[id] = xmlPenStyle(func(n string) string { v, _ := attr(def, n); return v })
			case name == "wp" && digitsRegexp.MatchString(ap):
				anchors[id] = intOf(ap)
			}
		}
	}
	return pens, anchors
}

// intOf parses digits, and gives a large value for a number that does not fit.
func intOf(digits string) int {
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 1 << 30
	}
	return n
}

func readParagraph(p *xmlraw.Node, pens map[string]style, anchors map[string]int) (caption, *model.ParseError) {
	t, perr := startAndEnd(p, "t", "d", time.Millisecond)
	if perr != nil {
		return caption{}, perr
	}
	segs, extras, perr := readSegments(p, pens, t.start)
	if perr != nil {
		return caption{}, perr
	}
	attrs := attributes(p)
	data := attrs.Clone()
	if len(extras) > 0 {
		data.Set("segments", extras)
	}
	c := caption{timing: t, segments: segs, data: data}
	if w, ok := attrs.String("w"); ok {
		c.window, c.hasWindow = w, true
	}
	if a, _ := attrs.String("a"); a == "1" {
		c.isAppend = true
		c.source = withText(attrs, p.Text())
	}
	wp, _ := attrs.String("wp")
	if anchor, ok := anchors[wp]; ok {
		c.alignment, _ = alignmentOf(anchor)
	}
	return c, nil
}

// readSegments returns the segments of a paragraph and the stored object of each: the attributes of a span, then
// its raw text. The text of a segment is decoded once more with the HTML5 entity table.
func readSegments(p *xmlraw.Node, pens map[string]style, start time.Duration) ([]segment, []any, *model.ParseError) {
	paragraphPen, _ := attr(p, "p")
	var segs []segment
	var extras []any
	for _, node := range p.Children {
		var raw string
		var span *xmlraw.Node
		switch node.Kind {
		case xmlraw.ElementNode:
			span, raw = node, node.Text()
		case xmlraw.TextNode, xmlraw.CDATANode, xmlraw.CommentNode, xmlraw.ProcInstNode:
			raw = node.Data
		case xmlraw.DocumentNode, xmlraw.DirectiveNode:
			continue
		}
		seg := segment{text: markupcore.DecodeEntities(raw)}
		extra := model.NewData()
		if span != nil {
			extra = attributes(span)
			offset, perr := spanOffset(span, start)
			if perr != nil {
				return nil, nil, perr
			}
			seg.offset, seg.hasOffset = offset.d, offset.ok
			pen, _ := attr(span, "p")
			if pen == "" || pen == "0" {
				pen = paragraphPen
			}
			seg.style = pens[pen]
		} else {
			seg.style = pens[paragraphPen]
		}
		segs = append(segs, seg)
		extras = append(extras, withText(extra, raw))
	}
	return segs, extras, nil
}

type optionalDuration struct {
	d  time.Duration
	ok bool
}

func spanOffset(span *xmlraw.Node, start time.Duration) (optionalDuration, *model.ParseError) {
	d, raw, state := attrTime(span, "t", time.Millisecond)
	switch state {
	case timeInvalid:
		return optionalDuration{}, invalidAttr(span, "t")
	case timeTooLarge:
		return optionalDuration{}, parsing.OverLimit(raw, span.Line)
	case timeValid:
		if _, err := parsing.BoundedTime(start+d, raw, span.Line); err != nil {
			return optionalDuration{}, asParseError(err)
		}
		return optionalDuration{d, true}, nil
	case timeAbsent:
	}
	return optionalDuration{}, nil
}
