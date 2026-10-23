package ass

import (
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// record is an ordered object of strings, such as one style or the fields of one event. A repeated name keeps
// its first place and its last value, as a PHP array key does.
type record struct {
	names, values []string
}

func (r *record) set(name, value string) {
	for i, n := range r.names {
		if n == name {
			r.values[i] = value
			return
		}
	}
	r.names = append(r.names, name)
	r.values = append(r.values, value)
}

// key returns the first name that equals name without ASCII case.
func (r *record) key(name string) (string, bool) {
	for _, n := range r.names {
		if equalFold(n, name) {
			return n, true
		}
	}
	return "", false
}

// field returns the value of the first name that equals name without ASCII case.
func (r *record) field(name string) (string, bool) {
	for i, n := range r.names {
		if equalFold(n, name) {
			return r.values[i], true
		}
	}
	return "", false
}

// lastField returns the value of the last name that equals name without ASCII case, as PHP
// array_change_key_case keeps it.
func (r *record) lastField(name string) string {
	for i := len(r.names) - 1; i >= 0; i-- {
		if equalFold(r.names[i], name) {
			return r.values[i]
		}
	}
	return ""
}

func (r *record) get(name string) (string, bool) {
	for i, n := range r.names {
		if n == name {
			return r.values[i], true
		}
	}
	return "", false
}

// data returns the record as format data. An empty record is an empty list, as PHP encodes an empty array.
func (r *record) data() any {
	if len(r.names) == 0 {
		return []any{}
	}
	d := model.NewData()
	for i, n := range r.names {
		d.Set(n, r.values[i])
	}
	return d
}

// recordOf reads a format data value that holds strings: an object by key, or a list by index.
func recordOf(v any) record {
	var r record
	switch v := v.(type) {
	case *model.Data:
		for _, k := range v.Keys() {
			s, _ := v.String(k)
			r.set(k, s)
		}
	case []any:
		for i, item := range v {
			s, _ := item.(string)
			r.set(strconv.Itoa(i), s)
		}
	}
	return r
}
