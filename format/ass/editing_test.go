package ass_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testRemoveDuplicateCuesKeepsAssEventsWithAnotherStyleOrLayer and testMergeSameTimeCuesRealAssFile of
// tests/CueEditingTest.php of subtitle-toolbox-php.
func TestEditingFixtures(t *testing.T) {
	t.Parallel()

	noBOM := model.CommonWriteOptions{BOM: model.BOMRemove}

	s := readFile(t, "editing/own_glow_duplicates.ass")
	require.NoError(t, s.RemoveDuplicateCues(0))
	assert.Equal(t, string(testfiles.Read(t, "editing/own_glow_duplicates_deduplicated.ass")), write(t, s, noBOM, ass.WriteOptions{}))

	s = readFile(t, "editing/own_same_time_speakers.ass")
	require.NoError(t, s.MergeSameTimeCues(0))
	var out bytes.Buffer
	require.NoError(t, srt.Write(&out, s, noBOM, srt.WriteOptions{}))
	assert.Equal(t, string(testfiles.Read(t, "editing/own_same_time_speakers_merged.srt")), out.String())
}
