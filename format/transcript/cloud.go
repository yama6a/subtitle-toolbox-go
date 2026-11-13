package transcript

import (
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// The helpers below are the same for the formats of the cloud speech-to-text services: Amazon Transcribe,
// Deepgram, AssemblyAI and Google Speech-to-Text.

// cloudRead reads r, runs the text pre-pass and decodes the JSON root object. ok is false for blank content,
// which reads as an empty subtitle.
func cloudRead(r io.Reader, f model.Format, common model.CommonReadOptions) (st *parsing.State, root orderedjson.Value, ok bool, err error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, root, false, fmt.Errorf("read %s: %w", f, err)
	}
	st = parsing.NewState(f, common)
	// The JSON decoder reads each bad byte as U+FFFD in both modes, so the UTF-8 check only warns.
	content, ok, err = st.Prepare(content, false)
	if err != nil || !ok {
		return st, root, false, err
	}
	root, err = orderedjson.Decode(content)
	if err != nil {
		msg := "The content is not valid JSON: " + err.Error() + "."
		return st, root, false, &model.ParseError{Format: f, Msg: msg, Err: err}
	}
	if root.Kind() != orderedjson.Object {
		return st, root, false, &model.ParseError{Format: f, Msg: "The JSON root must be an object."}
	}
	return st, root, true, nil
}

// cloudFail passes a field error to State.Fail. Strict mode returns it, lenient mode skips the item with a warning.
func cloudFail(st *parsing.State, err error, block int, item orderedjson.Value) error {
	var perr *model.ParseError
	if !errors.As(err, &perr) {
		return err
	}
	return st.Fail(perr, model.NoLine, block, []string{orderedjson.Block(item)})
}

func cloudData(v orderedjson.Value) *model.Data {
	d, _ := jsondata.ToData(v).(*model.Data)
	return d
}

func cloudGet(v orderedjson.Value, key string) orderedjson.Value {
	x, _ := v.Get(key)
	return x
}

// cloudStoredTime returns a number or a numeric string of seconds as whole milliseconds, or nil.
func cloudStoredTime(v orderedjson.Value) *time.Duration {
	if k := v.Kind(); k != orderedjson.Number && k != orderedjson.String {
		return nil
	}
	d, ok := v.Duration(time.Second)
	if !ok {
		return nil
	}
	return &d
}

// cloudConfidence returns a number from 0 to 1, else nil.
func cloudConfidence(v orderedjson.Value) *float64 {
	f, ok := v.AsFloat64()
	if !ok || math.IsNaN(f) || f < 0 || f > 1 {
		return nil
	}
	return &f
}

func cloudMember(key string, v orderedjson.Value) orderedjson.Member {
	return orderedjson.Member{Key: key, Value: v}
}

// cloudWrite writes the document that build returns as compact JSON without a BOM. label names the format in the
// error of a value that JSON cannot hold.
func cloudWrite(w io.Writer, s *model.Subtitle, f model.Format, label string, common model.CommonWriteOptions,
	build func(s *model.Subtitle) (orderedjson.Value, error),
) error {
	s, err := writing.SkipImageCues(s, f, common)
	if err != nil {
		return err
	}
	doc, err := build(s)
	if err != nil {
		return err
	}
	body, err := orderedjson.Encode(doc, false)
	if err != nil {
		reason := label + " cannot hold the subtitle: Inf and NaN cannot be JSON encoded."
		var cause error
		if errors.Is(err, orderedjson.ErrInvalidUTF8) {
			reason = label + " cannot hold the subtitle: Malformed UTF-8 characters, possibly incorrectly encoded. " +
				"Read a file in another encoding with its source encoding."
			cause = model.ErrInvalidUTF8
		}
		return &model.UnwritableError{Format: f, CueIndex: -1, Reason: reason, Err: cause}
	}
	return writing.Write(w, f, body, common, false)
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

// cloudCueData returns the format data of the cue as an object, or an empty object.
func cloudCueData(c *model.Cue, f model.Format) orderedjson.Value {
	d, ok := c.FormatData(string(f))
	if !ok {
		return orderedjson.ObjectOf()
	}
	v, err := jsondata.FromData(d)
	if err != nil || v.Kind() != orderedjson.Object {
		return orderedjson.ObjectOf()
	}
	return v
}
