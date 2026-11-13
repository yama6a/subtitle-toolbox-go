// Package jsondata converts between orderedjson values and the value tree of model.Data.
package jsondata

import (
	"fmt"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// ToData returns the model.Data value of v: nil, bool, int64, float64, string, []any or *model.Data.
// A number outside int64 or float64 range reads as PHP reads it: a float, with an infinity for 1e400.
func ToData(v orderedjson.Value) any {
	switch v.Kind() {
	case orderedjson.Bool:
		b, _ := v.AsBool()
		return b
	case orderedjson.Number:
		if i, ok := v.AsInt64(); ok {
			return i
		}
		text, _ := v.Text()
		f, _ := strconv.ParseFloat(text, 64)
		return f
	case orderedjson.String:
		s, _ := v.AsString()
		return s
	case orderedjson.Array:
		list := make([]any, len(v.Items()))
		for i, item := range v.Items() {
			list[i] = ToData(item)
		}
		return list
	case orderedjson.Object:
		d := model.NewData()
		for _, m := range v.Members() {
			d.Set(m.Key, ToData(m.Value))
		}
		return d
	case orderedjson.Null:
	}
	return nil
}

// FromData returns the Value of a model.Data value. It fails for a value outside the set of ToData.
func FromData(x any) (orderedjson.Value, error) {
	switch x := x.(type) {
	case nil:
		return orderedjson.Value{}, nil
	case bool:
		return orderedjson.BoolValue(x), nil
	case int64:
		return orderedjson.Int(x), nil
	case float64:
		return orderedjson.Float(x), nil
	case string:
		return orderedjson.StringValue(x), nil
	case []any:
		return fromList(x)
	case *model.Data:
		return fromObject(x)
	default:
		return orderedjson.Value{}, fmt.Errorf("format data value of type %T", x)
	}
}

func fromList(list []any) (orderedjson.Value, error) {
	items := make([]orderedjson.Value, len(list))
	for i, item := range list {
		var err error
		if items[i], err = FromData(item); err != nil {
			return orderedjson.Value{}, err
		}
	}
	return orderedjson.ArrayOf(items...), nil
}

func fromObject(d *model.Data) (orderedjson.Value, error) {
	members := make([]orderedjson.Member, 0, d.Len())
	for _, k := range d.Keys() {
		item, _ := d.Get(k)
		v, err := FromData(item)
		if err != nil {
			return orderedjson.Value{}, err
		}
		members = append(members, orderedjson.Member{Key: k, Value: v})
	}
	return orderedjson.ObjectOf(members...), nil
}
