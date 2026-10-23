package mpl2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/mpl2"
)

// Rows of the MPL2 signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"a cue", "[10][25]Hello|/world\n", true},
		{"brackets only", "[1][2]", true},
		{"a long run of digits", "[99999999999999999999][1]", true},
		{"a letter", "[1a][25]Hello\n", false},
		{"one time", "[1]", false},
		{"empty brackets", "[][2]x", false},
		{"a space between", "[1] [2]x", false},
		{"text before", "x[1][2]", false},
		{"an LRC timestamp", "[00:01.00]Hello\n", false},
		{"digits that are not ASCII", "[\u0661][\u0662]", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, mpl2.Detect([]byte(tt.content)))
		})
	}
}
