package stbox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
)

func TestLoadMatroska(t *testing.T) {
	t.Parallel()

	const path = "testdata/mkv/mkvmerge.mkv"
	tests := []struct {
		name      string
		opts      stbox.ReadOptions
		wantTrack int
		wantFmt   stbox.Format
		wantCues  int
	}{
		{"SubRip track", stbox.ReadOptions{Track: 2}, 2, stbox.SubRip, 6},
		{"ASS track", stbox.ReadOptions{Track: 3}, 3, stbox.ASS, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(path, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, stbox.ReadInfo{Format: tt.wantFmt, Container: stbox.Matroska, Track: tt.wantTrack}, info)
			assert.Equal(t, tt.wantCues, s.Len())
		})
	}

	t.Run("two tracks without a track number", func(t *testing.T) {
		t.Parallel()

		_, _, err := stbox.Load(path, stbox.ReadOptions{})
		var te *stbox.TrackError
		require.ErrorAs(t, err, &te)
		assert.Equal(t, stbox.Matroska, te.Container)
		assert.Len(t, te.Tracks, 2)
	})
}
