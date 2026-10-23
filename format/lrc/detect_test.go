package lrc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/lrc"
)

// Rows of the LRC signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"ID tag then time tag", "[ti:Morning Train]\n[00:12.00]Hello\n", true},
		{"time tag without fraction", "[00:12]Hello\n", true},
		{"two time tags", "[00:01.00]Hello\n[00:02.50]world\n", true},
		{"minutes only", "[00:01]Hello\n", true},
		{"hours", "[01:02:03]Hello\n", true},
		{"spaces in the tag", "[ 00:12.00 ]Hello\n", true},
		{"a time tag line after spaces", "[ar:Choir]\n \t[00:12.00]Hello\n", true},
		{"a comment first", "[#:note]\n[00:12.00]Hello\n", true},
		{"an empty ID tag", "[ti:]\n[00:12.00]Hello\n", true},
		{"only ID tags", "[ti:Morning Train]\n[ar:Choir]\n", false},
		{"text before", "Hello\n[00:12.00]Hello\n", false},
		{"4 minute digits", "[1234:45.67] Hello\n", false},
		{"61 seconds", "[00:61.00] Hello\n", false},
		{"4 fraction digits", "[00:12.0000] Hello\n", false},
		{"a comma before the fraction", "[00:12,00] Hello\n", false},
		{"hours with 75 minutes", "[1:75:00.00]Hello\n", false},
		{"MPL2", "[10][25]Hello|/world\n", false},
		{"an INI section", "[server:main]\nport=80\n", false},
		{"a time in the text", "Meet at [10:30] by the pier.\n[ferry] leaves at noon\n", false},
		{"an ID tag with LF in the value", "[ti:a\nb]\n[00:12.00]Hello\n", false},
		{"digits that are not ASCII", "[00:1\u0662]Hello\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, lrc.Detect([]byte(tt.content)))
		})
	}
}
