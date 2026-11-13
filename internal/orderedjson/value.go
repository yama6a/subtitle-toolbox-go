// Package orderedjson holds a JSON value that keeps the key order of objects and the source text of numbers.
package orderedjson

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Kind is the JSON type of a Value.
type Kind uint8

// The kinds of a Value. The zero Value is Null.
const (
	Null Kind = iota
	Bool
	Number
	String
	Array
	Object
)

// Value is a JSON value. Its zero value is null.
type Value struct {
	kind Kind
	// flag is the bool of a Bool value and marks a Number made from NaN or an infinity.
	flag    bool
	text    string
	items   []Value
	members []Member
}

// Member is one key and value of an object.
type Member struct {
	Key   string
	Value Value
}

// BoolValue returns a bool Value.
func BoolValue(b bool) Value { return Value{kind: Bool, flag: b} }

// StringValue returns a string Value.
func StringValue(s string) Value { return Value{kind: String, text: s} }

// Int returns the number Value of i.
func Int(i int64) Value { return Value{kind: Number, text: strconv.FormatInt(i, 10)} }

// Float returns the number Value of f in the layout of Python's json module. NaN and infinities fail at Encode.
func Float(f float64) Value {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return Value{kind: Number, flag: true, text: strconv.FormatFloat(f, 'g', -1, 64)}
	}
	return Value{kind: Number, text: formatFloat(f)}
}

// Seconds returns d in whole milliseconds as float seconds such as 4.0, 2.34 or 0.001. A part of a millisecond is cut off.
func Seconds(d time.Duration) Value {
	return Float(float64(d.Milliseconds()) / 1000)
}

// NumberText returns a number Value that keeps text as written. The text must be a JSON number literal.
func NumberText(text string) (Value, error) {
	if !validNumber(text) {
		return Value{}, ErrInvalidNumber
	}
	return Value{kind: Number, text: text}, nil
}

// ArrayOf returns an array Value.
func ArrayOf(items ...Value) Value { return Value{kind: Array, items: items} }

// ObjectOf returns an object Value. A repeated key keeps its first place and its last value.
func ObjectOf(members ...Member) Value {
	v := Value{kind: Object}
	var ix keyIndex
	for _, m := range members {
		v.members = ix.set(v.members, m)
	}
	return v
}

// Kind returns the JSON type of v.
func (v Value) Kind() Kind { return v.kind }

// AsBool returns the bool of a bool Value.
func (v Value) AsBool() (bool, bool) { return v.flag, v.kind == Bool }

// AsString returns the string of a string Value.
func (v Value) AsString() (string, bool) { return v.text, v.kind == String }

// Text returns the literal of a number Value.
func (v Value) Text() (string, bool) { return v.text, v.kind == Number }

// AsInt64 returns a number literal without '.', 'e' and 'E' that fits int64.
func (v Value) AsInt64() (int64, bool) {
	if v.kind != Number || v.flag || strings.ContainsAny(v.text, ".eE") {
		return 0, false
	}
	i, err := strconv.ParseInt(v.text, 10, 64)
	if err != nil {
		return 0, false
	}
	return i, true
}

// AsFloat64 returns the float64 of any finite number.
func (v Value) AsFloat64() (float64, bool) {
	if v.kind != Number || v.flag {
		return 0, false
	}
	f, err := strconv.ParseFloat(v.text, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// Items returns the items of an array. The caller must not change them.
func (v Value) Items() []Value { return v.items }

// Members returns the members of an object. The caller must not change them.
func (v Value) Members() []Member { return v.members }

// Get returns the value of key in an object.
func (v Value) Get(key string) (Value, bool) {
	for _, m := range v.members {
		if m.Key == key {
			return m.Value, true
		}
	}
	return Value{}, false
}

// Has reports whether an object has key, also when its value is null.
func (v Value) Has(key string) bool {
	_, ok := v.Get(key)
	return ok
}

// Union returns the PHP array union a + b: the members of a, then the members of b whose keys a lacks.
func Union(a, b Value) Value {
	out := Value{kind: Object, members: slices.Clone(a.members)}
	ix := newKeyIndex(out.members)
	for _, m := range b.members {
		if !ix.has(out.members, m.Key) {
			out.members = ix.set(out.members, m)
		}
	}
	return out
}

// Replace returns base with each argument applied in turn, as PHP array_replace does.
// A key that exists keeps its place and gets the new value. A new key goes last.
func Replace(base Value, with ...Value) Value {
	out := Value{kind: Object, members: slices.Clone(base.members)}
	ix := newKeyIndex(out.members)
	for _, w := range with {
		for _, m := range w.members {
			out.members = ix.set(out.members, m)
		}
	}
	return out
}

// With returns v with key set to x.
func (v Value) With(key string, x Value) Value {
	return Replace(v, ObjectOf(Member{Key: key, Value: x}))
}

// Without returns v without the members with keys.
func (v Value) Without(keys ...string) Value {
	out := Value{kind: Object}
	for _, m := range v.members {
		if !slices.Contains(keys, m.Key) {
			out.members = append(out.members, m)
		}
	}
	return out
}

// Equal reports whether a and b have the same kinds, the same keys in the same order and the same number text.
func Equal(a, b Value) bool {
	if a.kind != b.kind || a.flag != b.flag || a.text != b.text {
		return false
	}
	return slices.EqualFunc(a.items, b.items, Equal) &&
		slices.EqualFunc(a.members, b.members, func(x, y Member) bool { return x.Key == y.Key && Equal(x.Value, y.Value) })
}

// Canonical returns v with the members of every object sorted by key.
func Canonical(v Value) Value {
	out := v
	if v.items != nil {
		out.items = make([]Value, len(v.items))
		for i, item := range v.items {
			out.items[i] = Canonical(item)
		}
	}
	if v.members != nil {
		out.members = make([]Member, len(v.members))
		for i, m := range v.members {
			out.members[i] = Member{Key: m.Key, Value: Canonical(m.Value)}
		}
		slices.SortStableFunc(out.members, func(a, b Member) int { return strings.Compare(a.Key, b.Key) })
	}
	return out
}

// linearKeys is the member count up to which a key search scans the members instead of building a map.
const linearKeys = 16

// keyIndex finds the place of a key in a list of members. It builds a map once the list is long,
// so an object with n members costs O(n) to build and not O(n squared).
type keyIndex struct {
	byKey map[string]int
}

func newKeyIndex(members []Member) *keyIndex {
	ix := &keyIndex{}
	if len(members) > linearKeys {
		ix.build(members)
	}
	return ix
}

func (ix *keyIndex) build(members []Member) {
	ix.byKey = make(map[string]int, len(members)*2)
	for i, m := range members {
		ix.byKey[m.Key] = i
	}
}

func (ix *keyIndex) find(members []Member, key string) int {
	if ix.byKey != nil {
		if i, ok := ix.byKey[key]; ok {
			return i
		}
		return -1
	}
	return slices.IndexFunc(members, func(m Member) bool { return m.Key == key })
}

func (ix *keyIndex) has(members []Member, key string) bool { return ix.find(members, key) >= 0 }

// set puts m in members: an old key keeps its place and gets the new value, a new key goes last.
func (ix *keyIndex) set(members []Member, m Member) []Member {
	if i := ix.find(members, m.Key); i >= 0 {
		members[i].Value = m.Value
		return members
	}
	members = append(members, m)
	switch {
	case ix.byKey != nil:
		ix.byKey[m.Key] = len(members) - 1
	case len(members) > linearKeys:
		ix.build(members)
	}
	return members
}
