package youtube

import (
	"errors"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const xmlDeclaration = `<?xml version="1.0" encoding="utf-8" ?>`

var attrNameRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// Write writes s to w in the variant that the format field of the file format data names. It writes srv3 for a
// subtitle from another format. A caption that the user did not edit keeps the bytes of its source, except for the
// white space before the XML declaration and the indent of json3.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.YouTubeTimedText, common)
	if err != nil {
		return err
	}
	cues := s.Cues()
	stored := make([]orderedjson.Value, len(cues))
	for i, c := range cues {
		stored[i] = storedObject(c.FormatData(key))
	}
	file := storedObject(s.FormatData(key))
	variant, _ := file.Get("format")
	name := stringOf(variant)

	xw := &xmlWriter{}
	var body string
	switch name {
	case variantSrv1:
		body = xw.srv1(cues, stored)
	case variantSrv2:
		body = xw.srv2(cues, stored)
	case variantJSON3:
		return writeJSON3(w, json3Document(cues, stored, file), common)
	default:
		body = xw.srv3(cues, stored, file)
	}
	if xw.invalid {
		return &model.UnwritableError{
			Format: model.YouTubeTimedText, CueIndex: -1, Reason: xmlraw.InvalidUTF8Reason, Err: model.ErrInvalidUTF8,
		}
	}
	return writing.Write(w, model.YouTubeTimedText, []byte(body), common, false)
}

// storedObject returns the format data as a JSON object, an empty one for none.
func storedObject(d *model.Data, ok bool) orderedjson.Value {
	if !ok {
		return orderedjson.ObjectOf()
	}
	v, err := jsondata.FromData(d)
	if err != nil || v.Kind() != orderedjson.Object {
		return orderedjson.ObjectOf()
	}
	return v
}

type xmlWriter struct {
	invalid bool
}

// clean removes the characters that XML cannot hold. Text that is not valid UTF-8 marks the write as failed.
func (xw *xmlWriter) clean(s string) string {
	out, ok := xmlraw.Text(s)
	if !ok {
		xw.invalid = true
	}
	return out
}

//nolint:gochecknoglobals // the escape tables of XML text and attribute values
var (
	textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
)

func (xw *xmlWriter) escape(s string) string { return textEscaper.Replace(xw.clean(s)) }

// attributes writes the members of object, except the names in skip, as XML attributes. Only a name that XML
// allows with a string, number or bool value is written.
func (xw *xmlWriter) attributes(object orderedjson.Value, skip ...string) string {
	var b strings.Builder
	for _, m := range object.Members() {
		if !attrNameRegexp.MatchString(m.Key) || slices.Contains(skip, m.Key) {
			continue
		}
		var text string
		switch m.Value.Kind() {
		case orderedjson.String:
			text, _ = m.Value.AsString()
		case orderedjson.Number:
			text = numberText(m.Value)
		case orderedjson.Bool:
			if on, _ := m.Value.AsBool(); on {
				text = "1"
			}
		case orderedjson.Null, orderedjson.Array, orderedjson.Object:
			continue
		}
		b.WriteString(" " + m.Key + `="` + attrEscaper.Replace(xw.clean(text)) + `"`)
	}
	return b.String()
}

// numberText writes a number as PHP casts it to a string: an integer in decimal, a float in its shortest form
// without a trailing ".0".
func numberText(v orderedjson.Value) string {
	if i, ok := v.AsInt64(); ok {
		return strconv.FormatInt(i, 10)
	}
	f, ok := v.AsFloat64()
	switch {
	case !ok:
		return "INF"
	case math.Abs(f) < 1e15:
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'G', -1, 64)
}

// srv1 writes the transcript XML: one line, times in seconds, plain text.
func (xw *xmlWriter) srv1(cues []model.Cue, stored []orderedjson.Value) string {
	var b strings.Builder
	b.WriteString(xmlDeclaration + "<transcript>")
	for i, t := range (timeKeys{start: "start", duration: "dur", unit: time.Second, asString: true}).captions(cues, stored) {
		b.WriteString(xw.text(t.fields, cues[i]))
	}
	return b.String() + "</transcript>"
}

// srv2 writes one <text> per line, times in milliseconds, plain text.
func (xw *xmlWriter) srv2(cues []model.Cue, stored []orderedjson.Value) string {
	var b strings.Builder
	b.WriteString(xmlDeclaration + "\n<timedtext>\n")
	for i, t := range (timeKeys{start: "t", duration: "d", unit: time.Millisecond, asString: true}).captions(cues, stored) {
		b.WriteString(xw.text(t.fields, cues[i]) + "\n")
	}
	return b.String() + "</timedtext>\n"
}

// text returns the <text> element of a cue. A text that the source wrote with escaped entities keeps them.
func (xw *xmlWriter) text(fields orderedjson.Value, c model.Cue) string {
	text := strings.Join(markupcore.PlainLines(c.Lines), "\n")
	if v, _ := fields.Get("text"); v.Kind() == orderedjson.String {
		if raw, _ := v.AsString(); markupcore.DecodeEntities(raw) == text {
			text = raw
		}
	}
	return "<text" + xw.attributes(fields, "text", "appends", "segments") + ">" + xw.escape(text) + "</text>"
}

const (
	malformedUTF8 = "JSON cannot hold the subtitle: Malformed UTF-8 characters, possibly incorrectly encoded. " +
		"Read a file in another encoding with its source encoding."
	infOrNaN = "JSON cannot hold the subtitle: Inf and NaN cannot be JSON encoded."
)

func writeJSON3(w io.Writer, doc orderedjson.Value, common model.CommonWriteOptions) error {
	body, err := orderedjson.Encode(doc, false)
	if err != nil {
		reason := infOrNaN
		var cause error
		if errors.Is(err, orderedjson.ErrInvalidUTF8) {
			reason, cause = malformedUTF8, model.ErrInvalidUTF8
		}
		return &model.UnwritableError{Format: model.YouTubeTimedText, CueIndex: -1, Reason: reason, Err: cause}
	}
	return writing.Write(w, model.YouTubeTimedText, body, common, false)
}
