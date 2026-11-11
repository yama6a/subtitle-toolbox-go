package model_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func TestFrameRateFromFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fps  float64
		want model.FrameRate
		err  string
	}{
		{name: "25", fps: 25, want: model.FrameRate{Num: 25, Den: 1}},
		{name: "decimals stay exact", fps: 23.976, want: model.FrameRate{Num: 2997, Den: 125}},
		{name: "29.97", fps: 29.97, want: model.FrameRate{Num: 2997, Den: 100}},
		{name: "59.94", fps: 59.94, want: model.FrameRate{Num: 2997, Den: 50}},
		{name: "12.5", fps: 12.5, want: model.FrameRate{Num: 25, Den: 2}},
		{name: "0.5", fps: 0.5, want: model.FrameRate{Num: 1, Den: 2}},
		{name: "30000/1001 as a float", fps: 30000.0 / 1001, want: model.FrameRate{Num: 2997002997002997, Den: 100000000000000}},
		{name: "zero", fps: 0, err: "The frame rate must be greater than 0, got 0."},
		{name: "negative", fps: -25, err: "The frame rate must be greater than 0, got -25."},
		{name: "NaN", fps: math.NaN(), err: "The frame rate must be greater than 0, got NAN."},
		{name: "infinity", fps: math.Inf(1), err: "The frame rate must be greater than 0, got INF."},
		{name: "too many digits", fps: 1e300, err: "The frame rate must have at most 18 digits, got 1e+300."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := model.FrameRateFromFloat(tt.fps)
			if tt.err != "" {
				require.ErrorIs(t, err, model.ErrInvalidArgument)
				assert.Equal(t, &model.InvalidArgumentError{Msg: tt.err}, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.InDelta(t, tt.fps, got.Float64(), 0)
		})
	}
}

func TestFrameRateValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rate model.FrameRate
		err  string
	}{
		{name: "25", rate: model.FrameRate{Num: 25, Den: 1}},
		{name: "29.97 drop-frame", rate: model.FrameRate{Num: 30000, Den: 1001, DropFrame: true}},
		{name: "2997/100 drop-frame", rate: model.FrameRate{Num: 2997, Den: 100, DropFrame: true}},
		{name: "59.94 drop-frame", rate: model.FrameRate{Num: 60000, Den: 1001, DropFrame: true}},
		{name: "2997/50 drop-frame", rate: model.FrameRate{Num: 2997, Den: 50, DropFrame: true}},
		{name: "zero value", err: "The frame rate must be greater than 0, got 0."},
		{name: "negative", rate: model.FrameRate{Num: -1, Den: 1}, err: "The frame rate must be greater than 0, got -1."},
		{name: "zero denominator", rate: model.FrameRate{Num: 1}, err: "The frame rate must be greater than 0, got INF."},
		{
			name: "exactly 30 drop-frame", rate: model.FrameRate{Num: 30, Den: 1, DropFrame: true},
			err: "The frame rate must be from 29.5 to below 30 fps or from 59.5 to below 60 fps for drop-frame time codes, got 30.",
		},
		{
			name: "25 drop-frame", rate: model.FrameRate{Num: 25, Den: 1, DropFrame: true},
			err: "The frame rate must be from 29.5 to below 30 fps or from 59.5 to below 60 fps for drop-frame time codes, got 25.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.rate.Validate()
			if tt.err == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			assert.Equal(t, &model.InvalidArgumentError{Msg: tt.err}, err)
		})
	}
}

func TestFrameRateFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rate   model.FrameRate
		str    string
		isZero bool
	}{
		{rate: model.FrameRate{}, str: "0", isZero: true},
		{rate: model.FrameRate{Num: 25, Den: 1}, str: "25"},
		{rate: model.FrameRate{Num: 2997, Den: 125}, str: "23.976"},
		{rate: model.FrameRate{Num: 30000, Den: 1001}, str: "29.97"},
		{rate: model.FrameRate{Num: 1, Den: 3}, str: "0.333"},
		{rate: model.FrameRate{Num: 1000000, Den: 1}, str: "1000000"},
	}
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.str, tt.rate.String())
			assert.Equal(t, tt.isZero, tt.rate.IsZero())
		})
	}
}

func TestFrameRateEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b model.FrameRate
		want bool
	}{
		{name: "25/1 and 50/2", a: model.FrameRate{Num: 25, Den: 1}, b: model.FrameRate{Num: 50, Den: 2}, want: true},
		{name: "29.97 and 30000/1001", a: model.FrameRate{Num: 2997, Den: 100}, b: model.FrameRate{Num: 30000, Den: 1001}},
		{name: "drop-frame differs", a: model.FrameRate{Num: 2997, Den: 100}, b: model.FrameRate{Num: 2997, Den: 100, DropFrame: true}},
		{name: "zero values", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.a.Equal(tt.b))
			assert.Equal(t, tt.want, tt.b.Equal(tt.a))
		})
	}
}
