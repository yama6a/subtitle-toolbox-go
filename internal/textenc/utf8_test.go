package textenc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
)

func TestInvalidUTF8(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		offset  int
		replace string
	}{
		{name: "valid", in: "caf\u00e9 \U0001F600", offset: -1, replace: "caf\u00e9 \U0001F600"},
		{name: "empty", in: "", offset: -1, replace: ""},
		{name: "Latin-1 byte", in: "caf\xE9 noir", offset: 3, replace: "caf\ufffd noir"},
		{name: "one U+FFFD per bad byte", in: "a\xE9\xE9b", offset: 1, replace: "a\ufffd\ufffdb"},
		{name: "overlong form", in: "a\xC0\xAF", offset: 1, replace: "a\ufffd\ufffd"},
		{name: "surrogate", in: "\xED\xA0\x80", offset: 0, replace: "\ufffd\ufffd\ufffd"},
		{name: "above U+10FFFF", in: "\xF4\x90\x80\x80", offset: 0, replace: "\ufffd\ufffd\ufffd\ufffd"},
		{name: "cut sequence at the end", in: "ab\xE2\x82", offset: 2, replace: "ab\ufffd\ufffd"},
		{name: "real U+FFFD stays", in: "\ufffd\xFF", offset: 3, replace: "\ufffd\ufffd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.offset, textenc.InvalidUTF8Offset([]byte(tt.in)))
			assert.Equal(t, tt.replace, string(textenc.ReplaceInvalidUTF8([]byte(tt.in))))
		})
	}
}

func TestUTF8BOMs(t *testing.T) {
	t.Parallel()

	const bom = "\xEF\xBB\xBF"
	tests := []struct {
		name, in, trimmed, added string
	}{
		{name: "none", in: "a", trimmed: "a", added: bom + "a"},
		{name: "one", in: bom + "a", trimmed: "a", added: bom + "a"},
		{name: "two", in: bom + bom + "a", trimmed: "a", added: bom + bom + "a"},
		{name: "after the start", in: "a" + bom, trimmed: "a" + bom, added: bom + "a" + bom},
		{name: "empty", in: "", trimmed: "", added: bom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.trimmed, string(textenc.TrimUTF8BOMs([]byte(tt.in))))
			assert.Equal(t, tt.added, string(textenc.AddUTF8BOM([]byte(tt.in))))
		})
	}
}
