package stbox_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v2"
)

func TestOffsetRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		format     stbox.Format
		alignment  stbox.Alignment
		offset     stbox.Offset
		wantOffset *stbox.Offset
	}{
		{"TTML bottom center", stbox.TTML, stbox.BottomCenter, stbox.Offset{X: 50, Y: 95}, &stbox.Offset{X: 50, Y: 95}},
		{"TTML default anchor reads back as none", stbox.TTML, stbox.BottomCenter, stbox.Offset{X: 50, Y: 90}, nil},
		{"TTML top left", stbox.TTML, stbox.TopLeft, stbox.Offset{X: 10, Y: 5}, &stbox.Offset{X: 10, Y: 5}},
		{"TTML middle right", stbox.TTML, stbox.MiddleRight, stbox.Offset{X: 80, Y: 40}, &stbox.Offset{X: 80, Y: 40}},
		{"WebVTT bottom center", stbox.WebVTT, stbox.BottomCenter, stbox.Offset{X: 50, Y: 95}, &stbox.Offset{X: 50, Y: 95}},
		{"WebVTT top left keeps the column, not the height", stbox.WebVTT, stbox.TopLeft, stbox.Offset{X: 20, Y: 5}, &stbox.Offset{X: 20, Y: 0}},
		{"WebVTT top center keeps no offset", stbox.WebVTT, stbox.TopCenter, stbox.Offset{X: 50, Y: 10}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := stbox.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"One"}}
			c.Position = stbox.Position{Alignment: tt.alignment, Offset: &tt.offset}
			s := &stbox.Subtitle{}
			s.AddCues(c)

			var out bytes.Buffer
			_, err := stbox.Write(&out, s, tt.format, stbox.WriteOptions{})
			require.NoError(t, err)
			back, _, err := stbox.Parse(&out, stbox.ReadOptions{Format: tt.format})
			require.NoError(t, err)
			got, ok := back.Cue(0)
			require.True(t, ok)
			assert.Equal(t, tt.alignment, got.Position.Alignment)
			assert.Equal(t, tt.wantOffset, got.Position.Offset)
		})
	}
}
