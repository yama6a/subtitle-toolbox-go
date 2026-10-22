package sami_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/sami"
)

// Rows of the SAMI signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"upper case", "<SAMI>\n<HEAD>", true},
		{"lower case", "<sami><body></body></sami>", true},
		{"a space after the name", "<Sami lang=en>", true},
		{"the end after the name", "<SAMI", false},
		{"a longer name", "<SAMIPARAM>", false},
		{"XML declaration and comment", "<?xml version=\"1.0\"?>\n<!-- a\nb -->\n<SAMI>", true},
		{"DOCTYPE", "<!DOCTYPE html>\t<SAMI>", true},
		{"text first", "Converted online\n<SAMI>", false},
		{"U+017F for S", "<\u017FAMI>", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, sami.Detect([]byte(tt.content)))
		})
	}
}
