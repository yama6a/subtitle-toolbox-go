package sprucestl_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/sprucestl"
)

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"cue line", "00:00:01:10 , 00:00:03:10 , Bakery\n", true},
		{"cue line without spaces", "00:00:01:10,00:00:03:10,Bakery\n", true},
		{"cue line with an empty text", "00:00:01:10,00:00:03:10,", true},
		{"command lines before the cue", "$FadeOut\t=\t0\n \n00:00:01:10\t,\t00:00:03:10\t,\tBakery\n", true},
		{"comment line before a cue with a quote", "//Subtitles\n00:00:01:10 , 00:00:03:10 , \"Bakery\"\n", true},
		{"headerless CSV row", "00:00:01:10,00:00:03:10,\"Hello\"\n", false},
		{"headerless CSV row with spaces", "00:00:01:10 , 00:00:03:10 , \t\"Hello\"\n", false},
		{"3 hour digits", "100:00:01:10 , 100:00:03:10 , Bakery\n", false},
		{"minutes above 59", "00:60:01:10 , 00:60:03:10 , Bakery\n", false},
		{"commands without a cue", "$FadeOut = 0\n//note\n", false},
		{"a cue after a text line", "Bakery\n00:00:01:10 , 00:00:03:10 , Bakery\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, sprucestl.Detect([]byte(tt.content)))
		})
	}
}
