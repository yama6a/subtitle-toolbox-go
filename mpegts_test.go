package stbox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
)

func TestLoadMPEGTS(t *testing.T) {
	t.Parallel()

	s, info, err := stbox.Load(testfiles.Path(t, "mpegts/one_page.ts"), stbox.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, stbox.SubRip, info.Format)
	assert.Equal(t, stbox.MPEGTS, info.Container)
	assert.Equal(t, 888, info.Track)
	require.Len(t, s.Cues(), 2)
	assert.Equal(t, "The ferry leaves at noon.", s.Cues()[0].Text())

	_, _, err = stbox.Load(testfiles.Path(t, "mpegts/teletext.ts"), stbox.ReadOptions{})
	var te *stbox.TrackError
	require.ErrorAs(t, err, &te)
	assert.Len(t, te.Tracks, 5)

	s, _, err = stbox.Load(testfiles.Path(t, "mpegts/teletext.ts"), stbox.ReadOptions{Track: 777})
	require.NoError(t, err)
	assert.Len(t, s.Cues(), 2)
}
