package qttext_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/qttext"
)

// Rows of the QuickTime text signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"header line", "{QTtext} {font:Tahoma}\n[00:00:01.00]\n", true},
		{"header alone", "{QTtext}", true},
		{"lower case", "{qttext}{timeScale:30}\n", true},
		{"mixed case", "{qtTEXT}\n", true},
		{"no space before the next descriptor", "{QTtext}{timeScale:30}\n", true},
		{"a timestamp first", "[00:00:01.00]\nHello\n", false},
		{"a space before the brace", " {QTtext}\n", false},
		{"text before", "Hello\n{QTtext}\n", false},
		{"a value after the name", "{QTtext:1}\n", false},
		{"a longer name", "{QTtextual}\n", false},
		{"no closing brace", "{QTtext\n", false},
		{"MicroDVD", "{24}{72}Hello\n", false},
		{"U+212A is no letter of the name", "{QTtext\u212A}\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, qttext.Detect([]byte(tt.content)))
		})
	}
}
