package transcript

import (
	"fmt"
	"io"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// readRoot reads r, runs the text pre-pass and decodes the JSON root object of format f. ok is false for blank
// content, which reads as an empty subtitle.
func readRoot(r io.Reader, f model.Format, common model.CommonReadOptions) (st *parsing.State, root orderedjson.Value, ok bool, err error) {
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
	root, err = decodeObject(content, f)
	return st, root, err == nil, err
}

func decodeObject(content []byte, f model.Format) (orderedjson.Value, error) {
	v, err := orderedjson.Decode(content)
	if err != nil {
		msg := "The content is not valid JSON: " + err.Error() + "."
		return v, &model.ParseError{Format: f, Msg: msg, Err: err}
	}
	if v.Kind() != orderedjson.Object {
		return v, &model.ParseError{Format: f, Msg: "The JSON root must be an object."}
	}
	return v, nil
}

// storedObject returns the format data of the cue under key as a JSON object, or an empty object.
func storedObject(c *model.Cue, key string) orderedjson.Value {
	d, ok := c.FormatData(key)
	if !ok {
		return orderedjson.ObjectOf()
	}
	v, err := jsondata.FromData(d)
	if err != nil {
		return orderedjson.ObjectOf()
	}
	return v
}
