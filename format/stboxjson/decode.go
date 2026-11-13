package stboxjson

import (
	"encoding/json"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// decodeRoot decodes content into a tree of nil, bool, string, json.Number, []any and *model.Data values, and returns
// the root object. An empty list at the root reads as an empty object.
func decodeRoot(content []byte) (*model.Data, error) {
	v, err := orderedjson.Decode(content)
	if err != nil {
		msg := "The content is not valid JSON: " + err.Error() + "."
		return nil, &model.ParseError{Format: model.JSON, Msg: msg, Err: err}
	}
	if v.Kind() == orderedjson.Object {
		root, _ := tree(v).(*model.Data)
		return root, nil
	}
	if v.Kind() == orderedjson.Array && len(v.Items()) == 0 {
		return model.NewData(), nil
	}
	return nil, parseError("The JSON root must be an object.")
}

// tree converts v. A number stays a json.Number, so the writer prints the source literal.
func tree(v orderedjson.Value) any {
	switch v.Kind() {
	case orderedjson.Bool:
		b, _ := v.AsBool()
		return b
	case orderedjson.Number:
		text, _ := v.Text()
		return json.Number(text)
	case orderedjson.String:
		s, _ := v.AsString()
		return s
	case orderedjson.Array:
		list := make([]any, len(v.Items()))
		for i, item := range v.Items() {
			list[i] = tree(item)
		}
		return list
	case orderedjson.Object:
		obj := model.NewData()
		for _, m := range v.Members() {
			obj.Set(m.Key, tree(m.Value))
		}
		return obj
	case orderedjson.Null:
	}
	return nil
}

func parseError(msg string) *model.ParseError {
	return &model.ParseError{Format: model.JSON, Msg: msg}
}
