package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

type kv = model.Field

func TestDataKeepsInsertionOrder(t *testing.T) {
	t.Parallel()

	d := model.NewData(kv{Key: "b", Value: "1"}, kv{Key: "a", Value: "2"}, kv{Key: "b", Value: "3"})
	d.Set("c", int64(4))
	d.Set("a", "5")

	assert.Equal(t, []string{"b", "a", "c"}, d.Keys())
	assert.Equal(t, 3, d.Len())
	v, ok := d.Get("b")
	assert.True(t, ok)
	assert.Equal(t, "3", v)
	v, ok = d.Get("a")
	assert.True(t, ok)
	assert.Equal(t, "5", v)

	d.Delete("b")
	d.Delete("missing")
	assert.Equal(t, []string{"a", "c"}, d.Keys())
	_, ok = d.Get("b")
	assert.False(t, ok)

	d.Set("b", nil)
	assert.Equal(t, []string{"a", "c", "b"}, d.Keys())
	v, ok = d.Get("b")
	assert.True(t, ok)
	assert.Nil(t, v)
}

func TestDataKeysIsACopy(t *testing.T) {
	t.Parallel()

	d := model.NewData(kv{Key: "a", Value: "1"})
	d.Keys()[0] = "z"
	assert.Equal(t, []string{"a"}, d.Keys())
}

func TestNilDataIsEmpty(t *testing.T) {
	t.Parallel()

	var d *model.Data
	assert.Equal(t, 0, d.Len())
	assert.Empty(t, d.Keys())
	_, ok := d.Get("a")
	assert.False(t, ok)
	_, ok = d.String("a")
	assert.False(t, ok)
	d.Delete("a")
	assert.Nil(t, d.Clone())
}

func TestDataCloneIsDeep(t *testing.T) {
	t.Parallel()

	inner := model.NewData(kv{Key: "x", Value: "1"})
	list := []any{"a", model.NewData(kv{Key: "y", Value: "2"})}
	d := model.NewData(kv{Key: "inner", Value: inner}, kv{Key: "list", Value: list}, kv{Key: "none", Value: []any(nil)})
	c := d.Clone()
	assert.Equal(t, d, c)

	inner.Set("x", "changed")
	list[0] = "changed"
	nested, _ := list[1].(*model.Data)
	nested.Set("y", "changed")
	d.Set("new", true)

	want := model.NewData(
		kv{Key: "inner", Value: model.NewData(kv{Key: "x", Value: "1"})},
		kv{Key: "list", Value: []any{"a", model.NewData(kv{Key: "y", Value: "2"})}},
		kv{Key: "none", Value: []any(nil)},
	)
	assert.Equal(t, want, c)
}

func TestDataAccessors(t *testing.T) {
	t.Parallel()

	obj := model.NewData(kv{Key: "a", Value: "x"}, kv{Key: "b", Value: "y"})
	d := model.NewData(
		kv{Key: "str", Value: "text"},
		kv{Key: "int", Value: int64(7)},
		kv{Key: "float", Value: 2.5},
		kv{Key: "bool", Value: true},
		kv{Key: "list", Value: []any{"p", "q"}},
		kv{Key: "obj", Value: obj},
		kv{Key: "mixed", Value: []any{"p", int64(1)}},
		kv{Key: "null", Value: nil},
		kv{Key: "goint", Value: 7},
	)

	tests := []struct {
		name string
		get  func() (any, bool)
		want any
		ok   bool
	}{
		{"String", func() (any, bool) { return d.String("str") }, "text", true},
		{"String of an int", func() (any, bool) { return d.String("int") }, "", false},
		{"String of a missing key", func() (any, bool) { return d.String("missing") }, "", false},
		{"Int", func() (any, bool) { return d.Int("int") }, int64(7), true},
		{"Int of a float", func() (any, bool) { return d.Int("float") }, int64(0), false},
		{"Int of a Go int", func() (any, bool) { return d.Int("goint") }, int64(0), false},
		{"Number of an int", func() (any, bool) { return d.Number("int") }, 7.0, true},
		{"Number of a float", func() (any, bool) { return d.Number("float") }, 2.5, true},
		{"Number of a string", func() (any, bool) { return d.Number("str") }, 0.0, false},
		{"Bool", func() (any, bool) { return d.Bool("bool") }, true, true},
		{"Bool of null", func() (any, bool) { return d.Bool("null") }, false, false},
		{"List", func() (any, bool) { return d.List("list") }, []any{"p", "q"}, true},
		{"List of an object", func() (any, bool) { return d.List("obj") }, []any(nil), false},
		{"Object", func() (any, bool) { return d.Object("obj") }, obj, true},
		{"Object of a list", func() (any, bool) { return d.Object("list") }, (*model.Data)(nil), false},
		{"Strings of a list", func() (any, bool) { return d.Strings("list") }, []string{"p", "q"}, true},
		{"Strings of an object", func() (any, bool) { return d.Strings("obj") }, []string{"x", "y"}, true},
		{"Strings of a mixed list", func() (any, bool) { return d.Strings("mixed") }, []string(nil), false},
		{"Strings of a string", func() (any, bool) { return d.Strings("str") }, []string(nil), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := tt.get()
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
