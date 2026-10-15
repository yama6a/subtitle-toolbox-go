package webvtt_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const benchCues = 20000

// repeatCues returns n cues of source, copy k shifted by k periods. The period is the last end, rounded up to
// a whole second, plus 1 s.
func repeatCues(source *model.Subtitle, n int) *model.Subtitle {
	cues := source.Cues()
	period := cues[len(cues)-1].End.Truncate(time.Second)
	if period < cues[len(cues)-1].End {
		period += time.Second
	}
	period += time.Second
	out := make([]model.Cue, n)
	for i := range out {
		c := cues[i%len(cues)].Clone()
		offset := time.Duration(i/len(cues)) * period
		c.Start += offset
		c.End += offset
		out[i] = c
	}
	s := &model.Subtitle{}
	s.AddCues(out...)
	return s
}

// repeatWithNotes repeats the cues of harbour_tour.vtt every 30 s with a NOTE block before each cue,
// as testParsesTwentyThousandNoteBlocksUnderFiveSeconds of tests/CueListSpeedTest.php of subtitle-toolbox-php does.
func repeatWithNotes(tb testing.TB, n int) []byte {
	tb.Helper()

	content, err := os.ReadFile(filepath.Join(filepath.Dir(testfiles.Root(tb)), "model", "testdata", "editing", "harbour_tour.vtt"))
	require.NoError(tb, err)
	harbour, _, err := webvtt.Read(bytes.NewReader(content), model.CommonReadOptions{}, webvtt.ReadOptions{})
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

// BenchmarkParse reads 20,000 cues. The cues come from w3c_voices.vtt, because the SubRip source file of the other
// formats needs the SubRip reader.
func BenchmarkParse(b *testing.B) {
	b.Run("vtt", func(b *testing.B) {
		source, _, err := webvtt.Read(bytes.NewReader(testfiles.Read(b, "vtt/real/w3c_voices.vtt")), model.CommonReadOptions{}, webvtt.ReadOptions{})
		require.NoError(b, err)
		var content bytes.Buffer
		require.NoError(b, webvtt.Write(&content, repeatCues(source, benchCues), model.CommonWriteOptions{}, webvtt.WriteOptions{}))
		b.ReportAllocs()
		for b.Loop() {
			s, _, err := webvtt.Read(bytes.NewReader(content.Bytes()), model.CommonReadOptions{}, webvtt.ReadOptions{})
			if err != nil || s.Len() != benchCues {
				b.Fatalf("read %d cues, error %v", s.Len(), err)
			}
		}
	})
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
