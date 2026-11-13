package refsync_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/refsync"
)

// A transcript with times from the audio is a reference like any subtitle. The Whisper JSON of the English
// reference gives the same report as the SubRip file.
func TestApplyWithAWhisperReference(t *testing.T) {
	t.Parallel()

	english := load(t, "own_reference_en.srt")
	var transcript bytes.Buffer
	_, err := stbox.Write(&transcript, english, stbox.Whisper, stbox.WriteOptions{})
	require.NoError(t, err)
	reference, info, err := stbox.Parse(bytes.NewReader(transcript.Bytes()), int64(transcript.Len()), stbox.ReadOptions{})
	require.NoError(t, err)
	require.Equal(t, stbox.Whisper, info.Format)

	want, target := load(t, "own_target_de_25fps.srt"), load(t, "own_target_de_25fps.srt")
	wantReport, err := refsync.Apply(want, refsync.Options{Reference: english})
	require.NoError(t, err)
	got, err := refsync.Apply(target, refsync.Options{Reference: reference})
	require.NoError(t, err)
	assert.Equal(t, wantReport, got)
	assert.Equal(t, timesAndLines(want), timesAndLines(target))
}
