package model

import "slices"

// Data is the format data of one format: an ordered object of JSON-like values, where a nil *Data reads as empty.
type Data struct {
	keys   []string
	values map[string]any
}

// Field is one key and value, for NewData.
type Field struct {
	// Key is the name of the field.
	Key string
	// Value is nil, bool, int64, float64, string, []any or *Data.
	Value any
}

// NewData returns the data of fields in their order, where a repeated key keeps its first place and its last value.
func NewData(fields ...Field) *Data {
	d := &Data{}
	for _, f := range fields {
		d.Set(f.Key, f.Value)
	}
	return d
}

// Len returns the number of keys.
func (d *Data) Len() int {
	if d == nil {
		return 0
	}
	return len(d.keys)
}

// Keys returns the keys in insertion order.
func (d *Data) Keys() []string {
	if d == nil {
		return nil
	}
	return slices.Clone(d.keys)
}

// Get returns the value of key, or false when d has no such key.
func (d *Data) Get(key string) (any, bool) {
	if d == nil {
		return nil, false
	}
	v, ok := d.values[key]
	return v, ok
}

// Set sets the value of key on a non-nil d, where a new key goes last and an old key keeps its place.
func (d *Data) Set(key string, value any) {
	if d.values == nil {
		d.values = map[string]any{}
	}
	if _, ok := d.values[key]; !ok {
		d.keys = append(d.keys, key)
	}
	d.values[key] = value
}

// Delete removes key.
func (d *Data) Delete(key string) {
	if d == nil {
		return
	}
	if _, ok := d.values[key]; !ok {
		return
	}
	if len(d.keys) == 1 {
		d.keys, d.values = nil, nil
		return
	}
	delete(d.values, key)
	i := slices.Index(d.keys, key)
	d.keys = slices.Delete(d.keys, i, i+1)
}

// Clone returns a deep copy of d.
func (d *Data) Clone() *Data {
	if d == nil {
		return nil
	}
	c := &Data{keys: slices.Clone(d.keys)}
	if d.values != nil {
		c.values = make(map[string]any, len(d.values))
	}
	for k, v := range d.values {
		c.values[k] = cloneValue(v)
	}
	return c
}

func cloneValue(v any) any {
	switch v := v.(type) {
	case []any:
		if v == nil {
			return v
		}
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cloneValue(item)
		}
		return out
	case *Data:
		return v.Clone()
	default:
		return v
	}
}

// String returns the value of key when it is a string.
func (d *Data) String(key string) (string, bool) {
	v, _ := d.Get(key)
	s, ok := v.(string)
	return s, ok
}

// Int returns the value of key when it is an int64.
func (d *Data) Int(key string) (int64, bool) {
	v, _ := d.Get(key)
	n, ok := v.(int64)
	return n, ok
}

// Number returns the value of key when it is an int64 or a float64.
func (d *Data) Number(key string) (float64, bool) {
	v, _ := d.Get(key)
	switch n := v.(type) {
	case int64:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// Bool returns the value of key when it is a bool.
func (d *Data) Bool(key string) (bool, bool) {
	v, _ := d.Get(key)
	b, ok := v.(bool)
	return b, ok
}

// List returns the value of key when it is a []any.
func (d *Data) List(key string) ([]any, bool) {
	v, _ := d.Get(key)
	l, ok := v.([]any)
	return l, ok
}

// Object returns the value of key when it is a *Data.
func (d *Data) Object(key string) (*Data, bool) {
	v, _ := d.Get(key)
	o, ok := v.(*Data)
	return o, ok
}

// Strings returns the value of key when it is a list of strings, or the values in order when it is an object of strings.
func (d *Data) Strings(key string) ([]string, bool) {
	v, _ := d.Get(key)
	vals, ok := items(v)
	if !ok {
		return nil, false
	}
	out := make([]string, len(vals))
	for i, item := range vals {
		if out[i], ok = item.(string); !ok {
			return nil, false
		}
	}
	return out, true
}

// items returns the values of a list, or of an object in key order.
func items(v any) ([]any, bool) {
	switch v := v.(type) {
	case []any:
		return v, true
	case *Data:
		out := make([]any, v.Len())
		for i, k := range v.Keys() {
			out[i] = v.values[k]
		}
		return out, true
	default:
		return nil, false
	}
}
