package orderedjson_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
)

func TestDecodeErrors(t *testing.T) {
	t.Parallel()

	deep := func(n int) string { return strings.Repeat("[", n) + strings.Repeat("]", n) }
	tests := []struct {
		name  string
		input string
		msg   string
	}{
		{"empty", "", "Syntax error"},
		{"open object", "{", "Syntax error"},
		{"trailing data", `{} x`, "Syntax error"},
		{"trailing comma", `[1,]`, "Syntax error"},
		{"leading zero", `01`, "Syntax error"},
		{"bad escape", `"\x"`, "Syntax error"},
		{"bad hex", `"\uZZZZ"`, "Syntax error"},
		{"vertical tab outside a string", "\v1", "Control character error, possibly incorrectly encoded"},
		{"raw control in a string", "\"a\x01\"", "Control character error, possibly incorrectly encoded"},
		{"unterminated string", `"abc`, "Control character error, possibly incorrectly encoded"},
		{"high surrogate alone", `"\ud800"`, "Single unpaired UTF-16 surrogate in unicode escape"},
		{"high surrogate then letter", `"\ud800A"`, "Single unpaired UTF-16 surrogate in unicode escape"},
		{"low surrogate alone", `"\ude00"`, "Single unpaired UTF-16 surrogate in unicode escape"},
		{"bad byte outside a string", "[1,\xff]", "Malformed UTF-8 characters, possibly incorrectly encoded"},
		{"closer of the other kind", `[1}`, "State mismatch (invalid or malformed JSON)"},
		{"closer after a comma", `[1,}`, "Syntax error"},
		{"bad string after a value", `{"a":1"`, "Control character error, possibly incorrectly encoded"},
		{"depth 512 arrays", deep(512), "Maximum stack depth exceeded"},
		{"depth 512 open", strings.Repeat("[", 512), "Maximum stack depth exceeded"},
		{"depth 512 objects", strings.Repeat(`{"a":`, 511) + `{}` + strings.Repeat("}", 511), "Maximum stack depth exceeded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := orderedjson.Decode([]byte(tt.input))
			var serr *orderedjson.SyntaxError
			require.ErrorAs(t, err, &serr)
			assert.Equal(t, tt.msg, serr.Msg)
		})
	}

	t.Run("depth 511", func(t *testing.T) {
		t.Parallel()

		_, err := orderedjson.Decode([]byte(deep(511)))
		require.NoError(t, err)
	})
}

func TestDecodeValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"white space around", " \t\r\n[1, 2]\n", `[1,2]`},
		{"duplicate key keeps the first place", `{"a": 1, "b": 2, "a": 3}`, `{"a":3,"b":2}`},
		{"number text", `[0.0, -0, 1e400, 13.120000000000001, 1E5]`, `[0.0,-0,1e400,13.120000000000001,1E5]`},
		{"surrogate pair", `"\ud83d\ude00"`, "\"\U0001F600\""},
		{"escapes", `"\u0000\/\b\f\n\r\t\"\\"`, `"\u0000/\b\f\n\r\t\"\\"`},
		{"two bad bytes", "\"\xe9\x9b\"", "\"\ufffd\ufffd\""},
		{"one bad byte", "\"\xbb\"", "\"\ufffd\""},
		{"overlong bytes", "\"\xc0\x80\"", "\"\ufffd\ufffd\""},
		{"empty containers keep their kind", `{"a": {}, "b": []}`, `{"a":{},"b":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v, err := orderedjson.Decode([]byte(tt.input))
			require.NoError(t, err)
			out, err := orderedjson.Encode(v, false)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(out))
		})
	}
}

func TestDecodeLargeObject(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	b.WriteString("{")
	for i := range 3000 {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"k` + string(rune('a'+i%26)) + strings.Repeat("x", i/26) + `":1`)
	}
	b.WriteString(`,"ka":2}`)
	v, err := orderedjson.Decode([]byte(b.String()))
	require.NoError(t, err)
	assert.Len(t, v.Members(), 3000)
	got, _ := v.Get("ka")
	text, _ := got.Text()
	assert.Equal(t, "2", text)
	assert.Equal(t, "ka", v.Members()[0].Key)
}
