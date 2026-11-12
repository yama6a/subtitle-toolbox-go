package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func testImage() *model.Image {
	return &model.Image{PNG: []byte("png"), X: 1, Y: 2, Width: 3, Height: 4, ScreenWidth: 720, ScreenHeight: 576}
}

func TestImageValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		width, height int
		want          string
	}{
		{"zero side", 0, 1, "Cannot create a cue image of 0x1 pixels on a screen of 8000x8000 pixels: every size must be at least 1."},
		{"widest side", 7680, 1, ""},
		{"side too long", 7681, 1, "Cannot create a cue image of 7681x1 pixels on a screen of 8000x8000 pixels: an image of 7681x1 pixels is larger " +
			"than the limit of 7680 pixels per side and 8294400 pixels in total."},
		{"4K frame", 3840, 2160, ""},
		{"one row too many", 3840, 2161, "Cannot create a cue image of 3840x2161 pixels on a screen of 8000x8000 pixels: an image of 3840x2161 pixels is larger " +
			"than the limit of 7680 pixels per side and 8294400 pixels in total."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := model.Image{Width: tt.width, Height: tt.height, ScreenWidth: 8000, ScreenHeight: 8000}.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			var iae *model.InvalidArgumentError
			require.ErrorAs(t, err, &iae)
			assert.Equal(t, tt.want, iae.Msg)
		})
	}
}

func TestImageCueIsCopiedAndChecked(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{End: time.Second, Image: testImage()})

	got, _ := s.Cue(0)
	got.Image.PNG[0] = 'X'
	again, _ := s.Cue(0)
	assert.Equal(t, testImage(), again.Image)
	assert.Equal(t, testImage().PNG, s.Clone().Cues()[0].Image.PNG)

	got.Image.Width = 0
	require.ErrorIs(t, s.SetCue(0, got), model.ErrInvalidArgument)
}

func TestImageCuesAreNeverJoined(t *testing.T) {
	t.Parallel()

	build := func() *model.Subtitle {
		s := subtitleOf(cueSpec{1, 2, "Hello"}, cueSpec{2, 3, "Hello"})
		setCue(t, s, 1, func(c *model.Cue) { c.Image = testImage() })
		return s
	}
	tests := []struct {
		name string
		edit func(*model.Subtitle) error
	}{
		{"RemoveDuplicateCues", func(s *model.Subtitle) error { return s.RemoveDuplicateCues(time.Second) }},
		{"MergeSameTimeCues", func(s *model.Subtitle) error { return s.MergeSameTimeCues(2 * time.Second) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := build()
			require.NoError(t, tt.edit(s))
			assert.Equal(t, 2, s.Len())
		})
	}

	require.ErrorIs(t, build().JoinCues(0, 1), model.ErrInvalidArgument)
}
