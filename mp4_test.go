package stbox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

func TestLoadMP4(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
		opts stbox.ReadOptions
		cues int // 0 means at least one cue
		info stbox.ReadInfo
	}{
		{
			"the only subtitle track", "one_track.mp4",
			stbox.ReadOptions{},
			2,
			stbox.ReadInfo{Format: stbox.SubRip, Container: stbox.MP4, Track: 2},
		},
		{
			"a track by number", "text_tracks.mp4",
			stbox.ReadOptions{Track: 3},
			2,
			stbox.ReadInfo{Format: stbox.SubRip, Container: stbox.MP4, Track: 3},
		},
		{
			"the SCC roll-up option reaches a c608 track", "ffmpeg_c608_rollup.mov",
			stbox.ReadOptions{SCC: scc.ReadOptions{RollUp: scc.RollUpLines}},
			0,
			stbox.ReadInfo{Format: stbox.SCC, Container: stbox.MP4, Track: 1},
		},
		{
			"the MCC service option reaches a c708 track", "c708_track.mov",
			stbox.ReadOptions{MCC: mcc.ReadOptions{Service: 2}},
			0,
			stbox.ReadInfo{Format: stbox.MCC, Container: stbox.MP4, Track: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(testfiles.Path(t, "mp4/"+tt.file), tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.info, info)
			if tt.cues > 0 {
				assert.Len(t, s.Cues(), tt.cues)
			} else {
				assert.NotEmpty(t, s.Cues())
			}
		})
	}
}

func TestLoadMP4Errors(t *testing.T) {
	t.Parallel()

	_, _, err := stbox.Load(testfiles.Path(t, "mp4/text_tracks.mp4"), stbox.ReadOptions{})
	var te *stbox.TrackError
	require.ErrorAs(t, err, &te)
	assert.Len(t, te.Tracks, 4)
	assert.Equal(t, stbox.MP4, te.Container)
}
