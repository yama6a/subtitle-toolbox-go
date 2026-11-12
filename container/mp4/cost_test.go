package mp4_test

import (
	"bytes"
	"encoding/binary"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/container/mp4"
)

type countingReader struct {
	r     *bytes.Reader
	mu    sync.Mutex
	spans [][2]int64 // offset and length of each read
}

func (c *countingReader) ReadAt(p []byte, off int64) (int, error) {
	c.mu.Lock()
	c.spans = append(c.spans, [2]int64{off, int64(len(p))})
	c.mu.Unlock()
	return c.r.ReadAt(p, off)
}

func TestExtractDoesNotReadTheMediaDataOfOtherTracks(t *testing.T) {
	t.Parallel()

	const chunkSize, chunks = 16 << 10, 64
	video := videoTrack(1, 1, chunks, chunkSize)
	subtitle := trackSpec{
		id: 2, handler: "sbtl", entry: tx3gEntry(), perChunk: 1,
		samples: []sampleSpec{{3000, textSample("The bus leaves in 5 minutes.")}},
	}
	content := file([]trackSpec{video, subtitle}, true)
	videoStart := int64(bytes.Index(content, []byte("mdat")) + 4)
	videoEnd := videoStart + chunks*chunkSize
	moovStart := int64(bytes.Index(content, []byte("moov")) - 4)
	moovEnd := moovStart + int64(binary.BigEndian.Uint32(content[moovStart:]))

	r := &countingReader{r: bytes.NewReader(content)}
	f, err := mp4.Open(r, int64(len(content)))
	require.NoError(t, err)
	s := extract(t, f, 2, mp4.ExtractOptions{})

	assert.Equal(t, [][]string{{"The bus leaves in 5 minutes."}}, [][]string{s.Cues()[0].Lines})
	var total int64
	for _, span := range r.spans {
		off, n := span[0], span[1]
		total += n
		assert.False(t, off < videoEnd && off+n > videoStart, "a read of %d bytes at %d touches the video samples", n, off)
		inHeaders := off == 0 || off == moovStart
		assert.True(t, inHeaders || (off >= moovStart && off+n <= moovEnd) || off >= videoEnd, "unexpected read at %d", off)
	}
	assert.Less(t, total, int64(len(content)/4))
}
