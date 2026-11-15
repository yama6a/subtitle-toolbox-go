package transcript

import (
	"io"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/formatdata"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// readRoot reads r, runs the text pre-pass and decodes the JSON root object of format f. ok is false for blank
// content, which reads as an empty subtitle.
func readRoot(r io.Reader, f model.Format, common model.CommonReadOptions) (st *parsing.State, root orderedjson.Value, ok bool, err error) {
	// The JSON decoder reads each bad byte as U+FFFD in both modes, so the UTF-8 check only warns.
	var content []byte
	st, content, ok, err = parsing.ReadText(r, f, common, false)
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
	v, err := formatdata.ToJSON(d)
	if err != nil {
		return orderedjson.ObjectOf()
	}
	return v
}
