package position_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
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
	assTop := model.NewData(model.Field{Key: "text", Value: `{\an8\pos(10,10)}Hi`}, model.Field{Key: "alignment", Value: int64(8)})
	assNull := model.NewData(model.Field{Key: "text", Value: `{\fad(200,200)}Hi`}, model.Field{Key: "alignment", Value: nil})
	ttmlTop := model.NewData(model.Field{Key: "attributes", Value: model.NewData(model.Field{Key: "region", Value: "top"})}, model.Field{Key: "alignment", Value: int64(8)})
	tests := []struct {
		name      string
		data      *model.Data
		alignment model.Alignment
		want      bool
	}{
		{"WebVTT unchanged", topLeft, model.TopLeft, true},
		{"WebVTT changed", topLeft, model.BottomRight, false},
		{"WebVTT changed to the default", topLeft, model.BottomCenter, false},
		{"ASS unchanged", assTop, model.TopCenter, true},
		{"ASS changed", assTop, model.TopLeft, false},
		{"ASS null alignment, bottom center", assNull, model.BottomCenter, true},
		{"ASS null alignment, changed", assNull, model.TopCenter, false},
		{"TTML unchanged", ttmlTop, model.TopCenter, true},
		{"TTML changed", ttmlTop, model.BottomCenter, false},
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

func TestOffsetFits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		alignment model.Alignment
		offset    *model.Offset
		want      bool
	}{
		{"bottom center", model.BottomCenter, &model.Offset{X: 50, Y: 90}, true},
		{"top center, Y in the bottom row", model.TopCenter, &model.Offset{X: 50, Y: 90}, false},
		{"bottom center, X in the left column", model.BottomCenter, &model.Offset{X: 10, Y: 90}, false},
		{"top left", model.TopLeft, &model.Offset{X: 10, Y: 5}, true},
		{"33.333 is below 100/3", model.MiddleLeft, &model.Offset{X: 33.333, Y: 50}, true},
		{"66.7 is above 200/3", model.MiddleCenter, &model.Offset{X: 50, Y: 66.7}, false},
		{"bottom right corner", model.BottomRight, &model.Offset{X: 100, Y: 100}, true},
		{"outside 0 to 100", model.BottomCenter, &model.Offset{X: 50, Y: 101}, false},
		{"33.3334 rounds to 33.333, the left column", model.BottomCenter, &model.Offset{X: 33.3334, Y: 90}, false},
		{"66.6666 rounds to 66.667, the bottom row", model.MiddleCenter, &model.Offset{X: 50, Y: 66.6666}, false},
		{"NaN", model.BottomCenter, &model.Offset{X: math.NaN(), Y: 90}, false},
		{"nil", model.BottomCenter, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, position.OffsetFits(tt.alignment, tt.offset))
		})
	}
}
