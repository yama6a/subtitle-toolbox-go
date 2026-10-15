package stboxjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// maxDepth is the most lists and objects that PHP json_decode with its default depth of 512 reads inside each other.
const maxDepth = 511

var errDepth = errors.New("maximum stack depth exceeded")

// decodeRoot decodes content into a tree of nil, bool, string, json.Number, []any and *model.Data values, and returns
// the root object. An empty list at the root reads as an empty object.
func decodeRoot(content []byte) (*model.Data, error) {
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.UseNumber()
	root, err := decodeValue(dec, 0)
	if err == nil && len(bytes.TrimLeft(content[dec.InputOffset():], " \t\r\n")) > 0 {
		err = errors.New("text after the root value")
	}
	switch {
	case errors.Is(err, errDepth):
		return nil, parseError("The content is not valid JSON: Maximum stack depth exceeded.")
	case err != nil:
		// Token reports a truncated document as io.EOF. Unmarshal names the first syntax error as encoding/json users know it.
		var raw json.RawMessage
		if uerr := json.Unmarshal(content, &raw); uerr != nil {
			err = uerr
		}
		return nil, &model.ParseError{Format: model.JSON, Msg: "The content is not valid JSON: " + err.Error() + ".", Err: err}
	}
	switch root := root.(type) {
	case *model.Data:
		return root, nil
	case []any:
		if len(root) == 0 {
			return model.NewData(), nil
		}
	}
	return nil, parseError("The JSON root must be an object.")
}

// decodeValue reads the next value. Depth is the number of lists and objects around it.
func decodeValue(dec *json.Decoder, depth int) (any, error) {
	tok, err := token(dec)
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	if depth == maxDepth {
		return nil, errDepth
	}
	if delim == '{' {
		obj := model.NewData()
		for dec.More() {
			key, err := token(dec)
			if err != nil {
				return nil, err
			}
			v, err := decodeValue(dec, depth+1)
			if err != nil {
				return nil, err
			}
			obj.Set(key.(string), v) //nolint:forcetypeassert // Token returns an object key as a string or fails
		}
		_, err = token(dec)
		return obj, err
	}
	list := []any{}
	for dec.More() {
		v, err := decodeValue(dec, depth+1)
		if err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	_, err = token(dec)
	return list, err
}

func token(dec *json.Decoder) (json.Token, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	return tok, nil
}

func parseError(msg string) *model.ParseError {
	return &model.ParseError{Format: model.JSON, Msg: msg}
}
