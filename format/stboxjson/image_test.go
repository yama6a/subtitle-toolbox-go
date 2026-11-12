package stboxjson_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const imageFields = `"png": "x", "x": 0, "y": 0, "width": 2, "height": 1, "screenWidth": 10, "screenHeight": 10`

func TestReadImageForcedFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, cue string
		want      bool
	}{
		{"cue key only", `"forced": true, "formatData": {"image": {` + imageFields + `}}`, true},
		{"image key only", `"formatData": {"image": {` + imageFields + `, "forced": true}}`, true},
		{"neither", `"forced": false, "formatData": {"image": {` + imageFields + `, "forced": false}}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := mustRead(t, `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], `+tt.cue+`}]}`, false)
			c, ok := s.Cue(0)
			require.True(t, ok)
			require.NotNil(t, c.Image)
			assert.Equal(t, tt.want, c.Forced)
		})
	}
}

func TestReadBadImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, data, msg string
	}{
		{"missing size", `{"png": "x"}`, "The field cues[0].formatData.image.x must be an integer."},
		{"png number", `{"png": 1, "x": 0, "y": 0, "width": 2, "height": 1, "screenWidth": 10, "screenHeight": 10}`, "The field cues[0].formatData.image.png must be a string."},
		{"forced number", `{` + imageFields + `, "forced": 1}`, "The field cues[0].formatData.image.forced must be a boolean."},
		{
			"zero width", `{"png": "x", "x": 0, "y": 0, "width": 0, "height": 1, "screenWidth": 10, "screenHeight": 10}`,
			"The field cues[0].formatData.image is invalid. Cannot create a cue image of 0x1 pixels on a screen of 10x10 pixels: every size must be at least 1.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := `{"version": 1, "cues": [{"start": 1, "end": 2, "lines": [], "formatData": {"image": ` + tt.data + `}}]}`
			assert.Equal(t, tt.msg, strictMessage(t, content))
		})
	}
}
