package webvtt_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const benchCues = 20000

// repeatWithNotes repeats the cues of harbour_tour.vtt every 30 s with a NOTE block before each cue,
// as testParsesTwentyThousandNoteBlocksUnderFiveSeconds of tests/CueListSpeedTest.php of subtitle-toolbox-php does.
func repeatWithNotes(tb testing.TB, n int) []byte {
	tb.Helper()

	harbour, _, err := webvtt.Read(bytes.NewReader(testfiles.Read(tb, "editing/harbour_tour.vtt")), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(tb, err)
	cues, comments := harbour.Cues(), harbour.Comments()
	s := &model.Subtitle{}
	for i := range n {
		c := cues[i%len(cues)].Clone()
		offset := time.Duration(i/len(cues)) * 30 * time.Second
		c.Start += offset
		c.End += offset
		s.AddCue(c)
		require.NoError(tb, s.AddComment(comments[i%len(comments)].Text, i))
	}
	var out bytes.Buffer
	require.NoError(tb, webvtt.Write(&out, s, model.CommonWriteOptions{}, webvtt.WriteOptions{}))
	return out.Bytes()
}

func BenchmarkParseNotes(b *testing.B) {
	content := repeatWithNotes(b, benchCues)
	b.ReportAllocs()
	for b.Loop() {
		s, _, err := webvtt.Read(bytes.NewReader(content), model.CommonReadOptions{}, webvtt.ReadOptions{})
		if err != nil || len(s.Comments()) != benchCues {
			b.Fatalf("read %d comments, error %v", len(s.Comments()), err)
		}
	}
}

func TestParseNotesKeepsEveryComment(t *testing.T) {
	t.Parallel()

	content := repeatWithNotes(t, 200)
	s, _ := read(t, content, false)
	assert.Len(t, s.Comments(), 200)
	assert.Equal(t, string(content), write(t, s, model.CommonWriteOptions{}))
}
