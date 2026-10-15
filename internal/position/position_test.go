package position_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestAlignmentUnchanged(t *testing.T) {
	t.Parallel()

	topLeft := model.NewData(
		model.Field{Key: "line", Value: "0"},
		model.Field{Key: "position", Value: "30%"},
		model.Field{Key: "align", Value: "left"},
		model.Field{Key: "alignment", Value: int64(7)},
	)
	region := model.NewData(model.Field{Key: "region", Value: "fred"})
	tests := []struct {
		name      string
		data      *model.Data
		alignment model.Alignment
		want      bool
	}{
		{"WebVTT unchanged", topLeft, model.TopLeft, true},
		{"WebVTT changed", topLeft, model.BottomRight, false},
		{"WebVTT changed to the default", topLeft, model.BottomCenter, false},
		{"no alignment stored, bottom center", region, model.BottomCenter, true},
		{"no alignment stored, other", region, model.TopCenter, false},
		{"no data, bottom center", nil, model.BottomCenter, true},
		{"no data, other", nil, model.MiddleLeft, false},
		{"alignment of the wrong type counts as missing", model.NewData(model.Field{Key: "alignment", Value: "7"}), model.BottomCenter, true},
		{"alignment of the wrong type, other", model.NewData(model.Field{Key: "alignment", Value: 7.0}), model.TopLeft, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, position.AlignmentUnchanged(tt.data, tt.alignment))
		})
	}
}
