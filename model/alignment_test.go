package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestAlignmentGrid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		alignment model.Alignment
		name      string
		numpad    int
		row       int
		column    int
	}{
		{model.BottomCenter, "BottomCenter", 2, 0, 2},
		{model.BottomLeft, "BottomLeft", 1, 0, 1},
		{model.BottomRight, "BottomRight", 3, 0, 3},
		{model.MiddleLeft, "MiddleLeft", 4, 1, 1},
		{model.MiddleCenter, "MiddleCenter", 5, 1, 2},
		{model.MiddleRight, "MiddleRight", 6, 1, 3},
		{model.TopLeft, "TopLeft", 7, 2, 1},
		{model.TopCenter, "TopCenter", 8, 2, 2},
		{model.TopRight, "TopRight", 9, 2, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.name, tt.alignment.String())
			assert.Equal(t, tt.numpad, tt.alignment.Numpad())
			assert.Equal(t, tt.row, tt.alignment.Row())
			assert.Equal(t, tt.column, tt.alignment.Column())

			got, ok := model.AlignmentFromNumpad(tt.numpad)
			assert.True(t, ok)
			assert.Equal(t, tt.alignment, got)

			got, ok = model.AlignmentAt(tt.row, tt.column)
			assert.True(t, ok)
			assert.Equal(t, tt.alignment, got)
		})
	}
}

func TestAlignmentZeroValueIsBottomCenter(t *testing.T) {
	t.Parallel()

	var a model.Alignment
	assert.Equal(t, model.BottomCenter, a)
	assert.Equal(t, model.BottomCenter, model.Cue{}.Alignment)
}

func TestAlignmentOutsideTheGrid(t *testing.T) {
	t.Parallel()

	for _, n := range []int{-1, 0, 10} {
		a, ok := model.AlignmentFromNumpad(n)
		assert.False(t, ok, n)
		assert.Equal(t, model.BottomCenter, a, n)
	}

	cells := []struct{ row, column int }{{-1, 1}, {3, 1}, {0, 0}, {0, 4}, {1, -2}}
	for _, c := range cells {
		a, ok := model.AlignmentAt(c.row, c.column)
		assert.False(t, ok, c)
		assert.Equal(t, model.BottomCenter, a, c)
	}

	invalid := model.Alignment(42)
	assert.Equal(t, "Alignment(42)", invalid.String())
	assert.Zero(t, invalid.Numpad())
	assert.Zero(t, invalid.Row())
	assert.Zero(t, invalid.Column())
}
