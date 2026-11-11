package model_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func TestPositionRowOn(t *testing.T) {
	t.Parallel()

	scc := model.Grid{Rows: 15, Columns: 32}
	teletext := model.Grid{Rows: 23, Columns: 40}
	tests := []struct {
		name   string
		p      model.Position
		g      model.Grid
		lines  int
		want   int
		wantOK bool
	}{
		{"top row keeps its number", model.Position{Alignment: model.TopCenter, Row: 2, Grid: scc}, teletext, 1, 2, true},
		{"bottom row keeps its distance", model.Position{Row: 15, Grid: scc}, teletext, 1, 23, true},
		{"middle row scales", model.Position{Row: 8, Grid: scc}, teletext, 1, 12, true},
		{"2-line bottom caption", model.Position{Row: 14, Grid: scc}, teletext, 2, 22, true},
		{"a top row clamps to a smaller grid", model.Position{Alignment: model.TopLeft, Row: 5, Grid: scc}, model.Grid{Rows: 4}, 2, 3, true},
		{"lines taller than the grid", model.Position{Row: 15, Grid: scc}, model.Grid{Rows: 2}, 5, 1, true},
		{"bottom alignment keeps a middle row", model.Position{Row: 6, Grid: scc}, teletext, 1, 9, true},
		{"middle alignment on a middle row", model.Position{Alignment: model.MiddleCenter, Row: 8, Grid: scc}, teletext, 1, 12, true},
		{"top alignment on a bottom row", model.Position{Alignment: model.TopCenter, Row: 14, Grid: scc}, teletext, 1, 0, false},
		{"middle alignment on a bottom row", model.Position{Alignment: model.MiddleCenter, Row: 14, Grid: scc}, teletext, 1, 0, false},
		{"bottom alignment on a top row", model.Position{Row: 2, Grid: scc}, teletext, 1, 0, false},
		{"row outside the grid", model.Position{Row: 20, Grid: scc}, teletext, 1, 0, false},
		{"no row", model.Position{Grid: scc}, teletext, 1, 0, false},
		{"no grid", model.Position{Row: 1}, teletext, 1, 0, false},
		{"empty target grid", model.Position{Row: 1, Grid: scc}, model.Grid{}, 1, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := tt.p.RowOn(tt.g, tt.lines)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPositionEqualAndValidate(t *testing.T) {
	t.Parallel()

	grid := model.Grid{Rows: 15, Columns: 32}
	tests := []struct {
		name  string
		p     model.Position
		other model.Position
		equal bool
		err   string
	}{
		{name: "zero", equal: true},
		{name: "offsets compare by value", p: model.Position{Offset: &model.Offset{X: 1, Y: 2}}, other: model.Position{Offset: &model.Offset{X: 1, Y: 2}}, equal: true},
		{name: "different offsets", p: model.Position{Offset: &model.Offset{X: 1, Y: 2}}, other: model.Position{Offset: &model.Offset{X: 1, Y: 3}}},
		{name: "nil against zero offset", p: model.Position{Offset: &model.Offset{}}},
		{name: "different alignment", p: model.Position{Alignment: model.TopLeft}},
		{name: "row on a grid", p: model.Position{Row: 15, Column: 32, Grid: grid}, other: model.Position{Row: 15, Column: 32, Grid: grid}, equal: true},
		{name: "row above the grid", p: model.Position{Row: 16, Grid: grid}, err: "Position.Row"},
		{name: "grid without a row", p: model.Position{Grid: grid}, err: "Position.Grid"},
		{name: "negative grid", p: model.Position{Grid: model.Grid{Rows: -1}}, err: "Position.Grid"},
		{name: "negative column", p: model.Position{Column: -1, Grid: grid}, err: "Position.Column"},
		{name: "row without a grid", p: model.Position{Row: 1}, err: "Position.Row"},
		{name: "offset X below 0", p: model.Position{Offset: &model.Offset{X: -0.1}}, err: "Position.Offset.X"},
		{name: "offset Y NaN", p: model.Position{Offset: &model.Offset{Y: math.NaN()}}, err: "Position.Offset.Y"},
		{name: "offset 0 and 100", p: model.Position{Offset: &model.Offset{X: 0, Y: 100}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.equal, tt.p.Equal(tt.other))
			err := tt.p.Validate()
			if tt.err == "" {
				require.NoError(t, err)
				return
			}
			var iae *model.InvalidArgumentError
			require.ErrorAs(t, err, &iae)
			assert.Equal(t, tt.err, iae.Option)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
		})
	}
}

func TestSetCueValidatesThePosition(t *testing.T) {
	t.Parallel()

	var s model.Subtitle
	s.AddCue(model.Cue{Lines: []string{"a"}})
	offset := &model.Offset{X: 50, Y: 10}
	require.NoError(t, s.SetCue(0, model.Cue{Lines: []string{"a"}, Position: model.Position{Offset: offset}}))
	offset.Y = 20
	got, _ := s.Cue(0)
	assert.InDelta(t, 10.0, got.Position.Offset.Y, 0)

	err := s.SetCue(0, model.Cue{Lines: []string{"a"}, Position: model.Position{Row: 3}})
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	got, _ = s.Cue(0)
	assert.Equal(t, 0, got.Position.Row)
}
