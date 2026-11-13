package stbox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

func TestMatroskaDVBSubtitlesReadLikeTheTransportStream(t *testing.T) {
	t.Parallel()

	for _, track := range [][2]int{{2, 257}, {3, 258}} {
		fromMKV, info, err := stbox.Load(testfiles.Path(t, "mkv/dvbsub.mkv"), stbox.ReadOptions{Track: track[0]})
		require.NoError(t, err)
		assert.Equal(t, stbox.PGS, info.Format)
		fromTS, _, err := stbox.Load(testfiles.Path(t, "mpegts/dvb_subtitles.ts"), stbox.ReadOptions{Track: track[1]})
		require.NoError(t, err)
		assert.Equal(t, fromTS.Cues(), fromMKV.Cues())
	}
}
