package stboxjson

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	malformedUTF8 = "JSON cannot hold the subtitle: Malformed UTF-8 characters, possibly incorrectly encoded. " +
		"Read a file in another encoding with its source encoding."
	infOrNaN = "JSON cannot hold the subtitle: Inf and NaN cannot be JSON encoded."
)

var errInfOrNaN = errors.New("inf or NaN")

// document returns the library JSON of s. A string outside format data that is not valid UTF-8 fails first,
// because encoding/json would replace its bad bytes without an error.
func document(s *model.Subtitle, opts WriteOptions) ([]byte, error) {
	if i, ok := validText(s, !opts.OmitFormatData); !ok {
		return nil, &model.UnwritableError{Format: model.JSON, CueIndex: i, Reason: malformedUTF8, Err: model.ErrInvalidUTF8}
	}

	e := newEncoder()
	e.raw(`{"version":` + strconv.Itoa(Version) + `,"metadata":`)
	e.metadata(s.AllMetadata())
	e.raw(`,"comments":`)
	e.comments(s.Comments())
	if !opts.OmitFormatData {
		e.raw(`,"formatData":`)
		if err := e.formatData(s.FormatDataKeys(), s.FormatData); err != nil {
			return nil, &model.UnwritableError{Format: model.JSON, CueIndex: -1, Reason: infOrNaN}
		}
	}
	e.raw(`,"cues":[`)
	for i, c := range s.All() {
		e.comma(i)
		e.cue(c)
		if !opts.OmitFormatData {
			e.raw(`,"formatData":`)
			if err := e.formatData(c.FormatDataKeys(), c.FormatData); err != nil {
				return nil, &model.UnwritableError{Format: model.JSON, CueIndex: i, Reason: infOrNaN}
			}
		}
		e.raw("}")
	}
	e.raw("]}")

	if !opts.PrettyPrint {
		return e.out, nil
	}
	var out bytes.Buffer
	if err := json.Indent(&out, e.out, "", "    "); err != nil {
		return nil, fmt.Errorf("indent %s: %w", model.JSON, err)
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// validText returns false and the index of the first cue with text that is not valid UTF-8, or -1 for other text.
func validText(s *model.Subtitle, withFormatData bool) (int, bool) {
	for key, value := range s.AllMetadata() {
		if !utf8.ValidString(key) || !utf8.ValidString(value) {
			return -1, false
		}
	}
	if slices.ContainsFunc(s.Comments(), func(c model.Comment) bool { return !utf8.ValidString(c.Text) }) ||
		withFormatData && !validKeys(s.FormatDataKeys(), s.FormatData) {
		return -1, false
	}
	for i, c := range s.All() {
		if !validCueText(c, withFormatData) {
			return i, false
		}
	}
	return 0, true
}

func validCueText(c model.Cue, withFormatData bool) bool {
	return utf8.ValidString(c.ID) && !slices.ContainsFunc(c.Lines, func(l string) bool { return !utf8.ValidString(l) }) &&
		(!withFormatData || validKeys(c.FormatDataKeys(), c.FormatData))
}

func validKeys(keys []string, get func(string) (*model.Data, bool)) bool {
	for _, key := range keys {
		d, _ := get(key)
		if !utf8.ValidString(key) || !validValueKeys(d) {
			return false
		}
	}
	return true
}

func validValueKeys(v any) bool {
	switch v := v.(type) {
	case []any:
		return !slices.ContainsFunc(v, func(item any) bool { return !validValueKeys(item) })
	case *model.Data:
		for _, key := range v.Keys() {
			item, _ := v.Get(key)
			if !utf8.ValidString(key) || !validValueKeys(item) {
				return false
			}
		}
	}
	return true
}

// encoder builds compact JSON. Strings go through encoding/json without HTML escaping, so "/" and non-ASCII
// characters stay as they are. U+2028 and U+2029 are escaped, as PHP escapes them.
type encoder struct {
	out []byte
	str bytes.Buffer
	enc *json.Encoder
}

func newEncoder() *encoder {
	e := &encoder{}
	e.enc = json.NewEncoder(&e.str)
	e.enc.SetEscapeHTML(false)
	return e
}

// encodeEntry returns the compact JSON of a value of the decoded tree, for the block lines of a warning.
func encodeEntry(v any) string {
	e := newEncoder()
	_ = e.value(v) // the tree holds no float64, so no Inf or NaN
	return string(e.out)
}

func (e *encoder) raw(s string) {
	e.out = append(e.out, s...)
}

func (e *encoder) comma(i int) {
	if i > 0 {
		e.out = append(e.out, ',')
	}
}

func (e *encoder) string(s string) {
	e.str.Reset()
	_ = e.enc.Encode(s) //nolint:errchkjson // a string always encodes, and a bytes.Buffer never fails
	e.out = append(e.out, bytes.TrimSuffix(e.str.Bytes(), []byte("\n"))...)
}

func (e *encoder) metadata(metadata map[string]string) {
	e.raw("{")
	for i, key := range slices.Sorted(maps.Keys(metadata)) {
		e.comma(i)
		e.string(key)
		e.raw(":")
		e.string(metadata[key])
	}
	e.raw("}")
}

func (e *encoder) comments(comments []model.Comment) {
	e.raw("[")
	for i, c := range comments {
		e.comma(i)
		e.raw(`{"text":`)
		e.string(c.Text)
		e.raw(`,"beforeCueIndex":` + strconv.Itoa(c.BeforeCue) + "}")
	}
	e.raw("]")
}

func (e *encoder) cue(c model.Cue) {
	e.raw(`{"start":`)
	e.seconds(c.Start)
	e.raw(`,"end":`)
	e.seconds(c.End)
	e.raw(`,"lines":[`)
	for i, line := range c.Lines {
		e.comma(i)
		e.string(line)
	}
	e.raw(`],"identifier":`)
	if c.ID == "" {
		e.raw("null")
	} else {
		e.string(c.ID)
	}
	e.raw(`,"alignment":`)
	if c.Alignment == model.BottomCenter {
		e.raw("null")
	} else {
		e.raw(strconv.Itoa(c.Alignment.Numpad()))
	}
	if c.Forced {
		e.raw(`,"forced":true`)
	}
}

// seconds writes a time with whole milliseconds as seconds with at least 1 fraction digit, such as 4.0 or 1.001.
func (e *encoder) seconds(d time.Duration) {
	e.out = appendFloat(e.out, float64(d.Milliseconds())/1000)
}

func (e *encoder) formatData(keys []string, get func(string) (*model.Data, bool)) error {
	e.raw("{")
	for i, key := range keys {
		e.comma(i)
		e.string(key)
		e.raw(":")
		d, _ := get(key)
		if err := e.value(d); err != nil {
			return err
		}
	}
	e.raw("}")
	return nil
}

// value writes a format data value or a value of the decoded tree. A string that is not valid UTF-8 becomes
// {"base64": "..."}.
func (e *encoder) value(v any) error {
	switch v := v.(type) {
	case nil:
		e.raw("null")
	case bool:
		e.out = strconv.AppendBool(e.out, v)
	case int64:
		e.out = strconv.AppendInt(e.out, v, 10)
	case float64:
		return e.float(v)
	case json.Number:
		e.number(v)
	case string:
		e.binaryOrString(v)
	case []any:
		return e.list(v)
	case *model.Data:
		return e.object(v)
	}
	return nil
}

func (e *encoder) float(f float64) error {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return errInfOrNaN
	}
	e.out = appendFloat(e.out, f)
	return nil
}

// number writes a number literal as the reader reads it. A literal outside the float64 range keeps its text.
func (e *encoder) number(n json.Number) {
	if i, ok := integer(n); ok {
		e.out = strconv.AppendInt(e.out, i, 10)
	} else if f, _ := strconv.ParseFloat(string(n), 64); !math.IsInf(f, 0) {
		e.out = appendFloat(e.out, f)
	} else {
		e.raw(string(n))
	}
}

func (e *encoder) binaryOrString(s string) {
	if utf8.ValidString(s) {
		e.string(s)
		return
	}
	e.raw(`{"base64":`)
	e.string(base64.StdEncoding.EncodeToString([]byte(s)))
	e.raw("}")
}

func (e *encoder) list(l []any) error {
	e.raw("[")
	for i, item := range l {
		e.comma(i)
		if err := e.value(item); err != nil {
			return err
		}
	}
	e.raw("]")
	return nil
}

func (e *encoder) object(d *model.Data) error {
	e.raw("{")
	for i, key := range d.Keys() {
		e.comma(i)
		e.string(key)
		e.raw(":")
		item, _ := d.Get(key)
		if err := e.value(item); err != nil {
			return err
		}
	}
	e.raw("}")
	return nil
}

// appendFloat writes f as encoding/json writes a float64, and adds ".0" when the result has no "." and no exponent,
// so a float reads back as a float.
//
// Adapted from the Go standard library, src/encoding/json/encode.go, floatEncoder.encode
// (https://github.com/golang/go/blob/go1.27.2/src/encoding/json/encode.go), BSD-3-Clause.
func appendFloat(b []byte, f float64) []byte {
	start := len(b)
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	b = strconv.AppendFloat(b, f, format, -1, 64)
	if format == 'e' {
		if n := len(b); b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
		return b
	}
	if !bytes.ContainsRune(b[start:], '.') {
		b = append(b, ".0"...)
	}
	return b
}
