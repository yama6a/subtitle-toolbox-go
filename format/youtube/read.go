package youtube

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var xmlStartRegexp = regexp.MustCompile(`<(?:\?xml[\t\n\v\f\r ]|!--|!DOCTYPE[\t\n\v\f\r ]|(?:timedtext|transcript)[\t\n\v\f\r />])`)

// Read reads json3, srv3, srv2 and srv1 from r. The content decides the variant, and the reader stores its name as
// the format field of the file format data. A caption of json3 or srv3 with an error names the field, such as
// events[3].tStartMs. A caption of srv1 or srv2 names the element and the attribute.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.YouTubeTimedText, err)
	}
	st := parsing.NewState(model.YouTubeTimedText, common)
	// The XML variants replace a bad byte as TTML does. json3 reads it as U+FFFD in both modes, so the check only warns.
	content, ok, err := st.Prepare(content, !isJSON(content))
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	rd := &reader{st: st, wordTimes: opts.WordTimestamps}
	var s *model.Subtitle
	if isJSON(content) {
		s, err = rd.readJSON(content)
	} else {
		s, err = rd.readXML(string(content))
	}
	if err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

type reader struct {
	st        *parsing.State
	wordTimes bool
}

func (rd *reader) parseError(line int, msg string) *model.ParseError {
	return &model.ParseError{Format: model.YouTubeTimedText, Line: line, Msg: msg}
}

func (rd *reader) readXML(content string) (*model.Subtitle, error) {
	repaired, skipped := xmlraw.SkipLeadingText(content, xmlStartRegexp)
	if skipped != "" {
		line, lines, msg := xmlraw.LeadingText(content, skipped)
		if !rd.st.Lenient() {
			return nil, rd.parseError(line, msg)
		}
		rd.st.Warn(msg+" The parser skipped it.", line, model.NoBlock, lines, model.Repaired)
	}
	doc, err := xmlraw.Parse(strings.NewReader(repaired), xmlraw.ParseOptions{})
	if err != nil {
		return nil, rd.parseError(model.NoLine, "The content is not well-formed XML.")
	}
	root, _ := doc.Root()
	name := xmlraw.QName(root.Name)
	switch name {
	case "transcript":
		return rd.readTexts(root, variantSrv1, "start", "dur", time.Second)
	case "timedtext":
		if body := firstElement(root, "body"); body != nil {
			return rd.readSrv3(root, body)
		}
		return rd.readTexts(root, variantSrv2, "t", "d", time.Millisecond)
	}
	return nil, rd.parseError(model.NoLine, "The root element must be <timedtext> or <transcript>, not <"+name+">.")
}

func elements(n *xmlraw.Node) []*xmlraw.Node {
	var out []*xmlraw.Node
	for _, c := range n.Children {
		if c.Kind == xmlraw.ElementNode {
			out = append(out, c)
		}
	}
	return out
}

func firstElement(n *xmlraw.Node, name string) *xmlraw.Node {
	for _, c := range n.Children {
		if c.Kind == xmlraw.ElementNode && xmlraw.QName(c.Name) == name {
			return c
		}
	}
	return nil
}

func attr(n *xmlraw.Node, name string) (string, bool) {
	for _, a := range n.Attrs {
		if a.Name.Space == "" && a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

func attributes(n *xmlraw.Node) *model.Data {
	d := model.NewData()
	for _, a := range n.Attrs {
		if !xmlraw.IsNamespaceDecl(a) {
			d.Set(xmlraw.QName(a.Name), a.Value)
		}
	}
	return d
}

// withText returns d with text set under "text", unless d has an attribute of that name.
func withText(d *model.Data, text string) *model.Data {
	if _, ok := d.Get("text"); !ok {
		d.Set("text", text)
	}
	return d
}

type timeState uint8

const (
	timeAbsent timeState = iota
	timeInvalid
	timeTooLarge
	timeValid
)

// attrTime reads the attribute name of n in units of unit. A time must be numeric, finite and 0 or more.
func attrTime(n *xmlraw.Node, name string, unit time.Duration) (d time.Duration, raw string, state timeState) {
	raw, ok := attr(n, name)
	if !ok {
		return 0, "", timeAbsent
	}
	text, ok := orderedjson.NumericString(trimSpace(raw))
	if !ok {
		return 0, raw, timeInvalid
	}
	if f, _ := strconv.ParseFloat(text, 64); f < 0 || math.IsInf(f, 0) {
		return 0, raw, timeInvalid
	}
	d, ok = orderedjson.StringValue(text).Duration(unit)
	if !ok {
		return 0, raw, timeTooLarge
	}
	return d, raw, timeValid
}

func invalidAttr(n *xmlraw.Node, name string) *model.ParseError {
	return &model.ParseError{Line: n.Line, Msg: "The <" + xmlraw.QName(n.Name) + "> element has no valid \"" + name + "\" attribute."}
}

// startAndEnd reads the required start attribute and the optional duration attribute of a caption element.
func startAndEnd(n *xmlraw.Node, startName, durationName string, unit time.Duration) (timing, *model.ParseError) {
	start, raw, state := attrTime(n, startName, unit)
	switch state {
	case timeAbsent, timeInvalid:
		return timing{}, invalidAttr(n, startName)
	case timeTooLarge:
		return timing{}, parsing.OverLimit(raw, n.Line)
	case timeValid:
	}
	t := timing{start: start}
	dur, raw, state := attrTime(n, durationName, unit)
	switch state {
	case timeInvalid:
		return timing{}, invalidAttr(n, durationName)
	case timeTooLarge:
		return timing{}, parsing.OverLimit(raw, n.Line)
	case timeValid:
		t.end, t.hasEnd = start+dur, true
		if _, err := parsing.BoundedTime(t.end, raw, n.Line); err != nil {
			return timing{}, asParseError(err)
		}
	case timeAbsent:
	}
	return t, nil
}

func asParseError(err error) *model.ParseError {
	perr, _ := err.(*model.ParseError) //nolint:errorlint // BoundedTime returns the type itself
	return perr
}

func (rd *reader) failElement(perr *model.ParseError, n *xmlraw.Node, block int) error {
	var b strings.Builder
	_ = xmlraw.Serialize(&b, n)
	return rd.st.Fail(perr, n.Line, block, []string{b.String()})
}

// readTexts reads srv1 and srv2: one caption per child element of the root, with no styles and no word times.
func (rd *reader) readTexts(root *xmlraw.Node, variant, startName, durationName string, unit time.Duration) (*model.Subtitle, error) {
	var caps []caption
	for i, n := range elements(root) {
		t, perr := startAndEnd(n, startName, durationName, unit)
		if perr != nil {
			if err := rd.failElement(perr, n, i); err != nil {
				return nil, err
			}
			continue
		}
		raw := n.Text()
		text := markupcore.DecodeEntities(raw)
		data := attributes(n)
		if text != raw {
			withText(data, raw)
		}
		caps = append(caps, caption{timing: t, segments: []segment{{text: text}}, data: data})
	}
	return assemble(rd.st, caps, model.NewData(model.Field{Key: "format", Value: variant}), false)
}
