package tmplayer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/tmplayer"
)

// Rows of the TMPlayer signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"colon", "00:00:01:Hello|world\n", true},
		{"equals sign", "0:00:01=Hello\n", true},
		{"line number and equals sign", "00:00:01,1=Hello\n", true},
		{"line number and colon", "00:00:01,12:Hello\n", true},
		{"hours with 3 digits", "100:00:01:Hello\n", true},
		{"an end entry", "00:00:04:\n", true},
		{"a number in the text", "00:00:01:5 trains leave\n", true},
		{"3 digits after the colon", "00:00:01:123, trains\n", true},
		{"a digit and a space after the colon", "00:00:01:5 trains, leave\n", true},
		{"a number at the end", "00:00:01:5", true},
		{"a time with a colon at the end of the content", "00:00:01:", true},
		{"Spruce STL without spaces", "00:00:01:10,00:00:03:10,Hello\n", false},
		{"Spruce STL with spaces", "00:00:01:10 , 00:00:03:10 , Hello\n", false},
		{"Spruce STL with tabs", "00:00:01:10\t,\t00:00:03:10\t,\tHello\n", false},
		{"a frame timecode with 1 digit", "00:00:01:1,", false},
		{"two frame timecodes", "00:00:01:10:00:00:03:10 Hello\n", false},
		{"a longer frame timecode", "00:00:01:100:00:03 Hello\n", false},
		{"a frame timecode that is cut", "00:00:01:10:00:0\n", true},
		{"60 minutes", "00:60:01:Hello\n", false},
		{"60 seconds", "00:00:60:Hello\n", false},
		{"a dot", "00:00:01.500:Hello\n", false},
		{"a comma without digits", "00:00:01,=Hello\n", false},
		{"no separator", "00:00:01 Hello\n", false},
		{"only the time", "00:00:01", false},
		{"text before", " 00:00:01:Hello", false},
		{"SBV", "0:00:01.500,0:00:04.000\nHello\n", false},
		{"digits that are not ASCII", "00:00:0\u0661:Hello", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tmplayer.Detect([]byte(tt.content)))
		})
	}
}
