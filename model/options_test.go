package model_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestOptionEnumStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value fmt.Stringer
		want  string
	}{
		{model.LF, "lf"},
		{model.CRLF, "crlf"},
		{model.LineEnding(2), "LineEnding(2)"},
		{model.BOMDefault, "default"},
		{model.BOMAdd, "add"},
		{model.BOMRemove, "remove"},
		{model.BOM(3), "BOM(3)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.value.String())
		})
	}
}
