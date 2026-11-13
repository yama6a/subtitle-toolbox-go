package orderedjson_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/orderedjson"
)

func TestEncodeStrings(t *testing.T) {
	t.Parallel()

	in := "\"\\\b\f\n\r\t\x01\x1f/<>&\x7f\u00e9\u2028\u2029"
	want := `"\"\\\b\f\n\r\t\u0001\u001f/<>&` + "\x7f\u00e9" + `\u2028\u2029"`
	out, err := orderedjson.Encode(orderedjson.StringValue(in), false)
	require.NoError(t, err)
	assert.Equal(t, want, string(out))

	_, err = orderedjson.Encode(orderedjson.StringValue("a\xffb"), false)
	require.ErrorIs(t, err, orderedjson.ErrInvalidUTF8)
	_, err = orderedjson.Encode(orderedjson.ObjectOf(orderedjson.Member{Key: "\xff"}), false)
	require.ErrorIs(t, err, orderedjson.ErrInvalidUTF8)
}

func TestEncodePretty(t *testing.T) {
	t.Parallel()

	v, err := orderedjson.Decode([]byte(`{"a":[1,{"b":null,"c":[]}],"d":{},"e":"x","f":true}`))
	require.NoError(t, err)
	out, err := orderedjson.Encode(v, true)
	require.NoError(t, err)
	want := `{
    "a": [
        1,
        {
            "b": null,
            "c": []
        }
    ],
    "d": {},
    "e": "x",
    "f": true
}
`
	assert.Equal(t, want, string(out))
	out, err = orderedjson.Encode(orderedjson.ArrayOf(), true)
	require.NoError(t, err)
	assert.Equal(t, "[]\n", string(out))
}

func TestBlock(t *testing.T) {
	t.Parallel()

	v, err := orderedjson.Decode([]byte(`{"id": 0, "text": "a", "avg_logprob": 1e400, "no_speech_prob": 0.0}`))
	require.NoError(t, err)
	assert.Equal(t, `{"id":0,"text":"a","avg_logprob":1e400,"no_speech_prob":0.0}`, orderedjson.Block(v)) //nolint:testifylint // JSONEq does not compare number text
}

func TestObjectEdits(t *testing.T) {
	t.Parallel()

	obj := func(kv ...string) orderedjson.Value {
		var ms []orderedjson.Member
		for i := 0; i < len(kv); i += 2 {
			n, err := orderedjson.NumberText(kv[i+1])
			require.NoError(t, err)
			ms = append(ms, orderedjson.Member{Key: kv[i], Value: n})
		}
		return orderedjson.ObjectOf(ms...)
	}
	text := func(v orderedjson.Value) string {
		out, err := orderedjson.Encode(v, false)
		require.NoError(t, err)
		return string(out)
	}

	assert.Equal(t, `{"x":1,"y":2,"z":3}`, text(orderedjson.Union(obj("x", "1", "y", "2"), obj("y", "9", "z", "3"))))
	assert.Equal(t, `{"id":7,"start":2,"text":5}`, text(orderedjson.Replace(obj("id", "1", "start", "2"), obj("text", "5", "id", "7"))))
	assert.Equal(t, `{"id":1,"start":9,"x":4}`, text(obj("id", "1", "start", "2").With("start", orderedjson.Int(9)).With("x", orderedjson.Int(4))))
	assert.Equal(t, `{"b":2}`, text(obj("a", "1", "b", "2", "c", "3").Without("a", "c", "d")))

	base := obj("a", "1")
	_ = base.With("a", orderedjson.Int(2))
	assert.Equal(t, `{"a":1}`, text(base))
}

func TestUnionLarge(t *testing.T) {
	t.Parallel()

	a, b := make([]orderedjson.Member, 0, 100), make([]orderedjson.Member, 0, 200)
	for i := range 100 {
		a = append(a, orderedjson.Member{Key: "a" + string(rune('A'+i%26)) + string(rune('a'+i/26))})
		b = append(b, orderedjson.Member{Key: "a" + string(rune('A'+i%26)) + string(rune('a'+i/26))}, orderedjson.Member{Key: "n" + string(rune('A'+i%26)) + string(rune('a'+i/26))})
	}
	u := orderedjson.Union(orderedjson.ObjectOf(a...), orderedjson.ObjectOf(b...))
	assert.Len(t, u.Members(), 200)
	assert.True(t, u.Has("nAa"))
}

func TestEqualAndCanonical(t *testing.T) {
	t.Parallel()

	a, err := orderedjson.Decode([]byte(`{"b": [1.0, {"y": 1, "x": 2}], "a": null}`))
	require.NoError(t, err)
	b, err := orderedjson.Decode([]byte(`{"a": null, "b": [1.0, {"x": 2, "y": 1}]}`))
	require.NoError(t, err)
	c, err := orderedjson.Decode([]byte(`{"a": null, "b": [1, {"x": 2, "y": 1}]}`))
	require.NoError(t, err)
	assert.False(t, orderedjson.Equal(a, b))
	assert.True(t, orderedjson.Equal(orderedjson.Canonical(a), orderedjson.Canonical(b)))
	assert.False(t, orderedjson.Equal(orderedjson.Canonical(b), orderedjson.Canonical(c)))
	assert.True(t, b.Has("a"))
}
