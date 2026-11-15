package markupcore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColorHexWithAlpha(t *testing.T) {
	t.Parallel()

	tests := []struct {
		color  string
		want   string
		wantOK bool
	}{
		{"#FF000080", "#ff000080", true},
		{"rgba(255, 0, 0, 0.5)", "#ff000080", true},
		{"rgba(255,0,0,0.5)", "#ff000080", true},
		{"rgba(255,0,0,50%)", "#ff000080", true},
		{"RGBA(255,0,0,128)", "#ff000080", true},
		{"rgba(255,0,0,1)", "#ff000001", true},
		{"rgba(255,0,0,.5)", "#ff000080", true},
		{"rgba(255,0,0,255)", "#ff0000ff", true},
		{"rgba(0,0,0,0)", "#00000000", true},
		{"rgb(1,2,3,4)", "#01020304", true},
		{"rgba(1,2,3)", "#010203", true},
		{"#ff0000zz", "#ff0000", true},
		{"#ff000080f", "#ff0000", true},
		{"red", "#ff0000", true},
		{"RED", "#ff0000", true},
		{"#FFF", "#ffffff", true},
		{" &#X23;FF0 ", "#ffff00", true},
		{"rgba(255,0,0,1.5)", "", false},
		{"rgba(255,0,0,5.)", "", false},
		{"rgba(255,0,0,100.2%)", "", false},
		{"rgba(255,0,0,256)", "", false},
		{"rgba(1,2,3,9223372036854775808)", "", false},
		{"rgb(256,0,0)", "", false},
		{"rgb(1000,0,0)", "", false},
		{"#ff00", "", false},
		{"transparent", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.color, func(t *testing.T) {
			t.Parallel()

			got, ok := colorHexWithAlpha(tt.color)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
