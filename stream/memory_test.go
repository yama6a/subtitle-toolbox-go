package stream_test

import (
	"bufio"
	"io"
	"iter"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/stream"
)

const (
	memoryCues  = 200000
	memoryLimit = 2 << 20
	sampleEvery = 512
)

// heapObjects reads the bytes of the heap objects, live and not yet swept.
func heapObjects() uint64 {
	sample := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(sample)
	return sample[0].Value.Uint64()
}

// heapGrowth runs fn, which calls sample now and then, and returns the highest heap size that sample saw
// above the heap size before fn.
func heapGrowth(fn func(sample func())) uint64 {
	runtime.GC()
	before := heapObjects()
	var peak uint64
	fn(func() { peak = max(peak, heapObjects()) })
	return peak - min(peak, before)
}

type cueWriter interface {
	Write(c model.Cue) error
	Close() error
}

// Ported from tests/Streaming/StreamMemoryTest.php. It does not run in parallel, because it measures the
// heap of the whole process. A test that calls t.Parallel waits until the tests in sequence are done.
func TestStreamsManyCuesInConstantMemory(t *testing.T) { //nolint:paralleltest // see above
	if testing.Short() {
		t.Skip("writes 200000 cues to a file")
	}
	defer debug.SetGCPercent(debug.SetGCPercent(10))

	tests := []struct {
		name      string
		newWriter func(io.Writer) (cueWriter, error)
		newReader func(io.Reader) iter.Seq2[model.Cue, error]
	}{
		{
			name: "SubRip",
			newWriter: func(w io.Writer) (cueWriter, error) {
				return stream.NewSubRipWriter(w, model.CommonWriteOptions{}, srt.WriteOptions{})
			},
			newReader: func(r io.Reader) iter.Seq2[model.Cue, error] {
				return stream.NewSubRipReader(r, stream.ReadOptions{}).Cues()
			},
		},
		{
			name: "WebVTT",
			newWriter: func(w io.Writer) (cueWriter, error) {
				return stream.NewWebVTTWriter(w, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
			},
			newReader: func(r io.Reader) iter.Seq2[model.Cue, error] {
				return stream.NewWebVTTReader(r, stream.ReadOptions{}).Cues()
			},
		},
	}
	for _, tt := range tests { //nolint:paralleltest // see above
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Create(filepath.Join(t.TempDir(), "many."+tt.name))
			require.NoError(t, err)
			defer f.Close()

			var writeErr error
			growth := heapGrowth(func(sample func()) {
				bw := bufio.NewWriter(f)
				w, err := tt.newWriter(bw)
				if err != nil {
					writeErr = err
					return
				}
				for i := range memoryCues {
					if writeErr = w.Write(memoryCue(i)); writeErr != nil {
						return
					}
					if i%sampleEvery == 0 {
						sample()
					}
				}
				if writeErr = w.Close(); writeErr == nil {
					writeErr = bw.Flush()
				}
			})
			require.NoError(t, writeErr)
			assert.Less(t, growth, uint64(memoryLimit), "write")
			info, err := f.Stat()
			require.NoError(t, err)
			assert.Greater(t, info.Size(), int64(10<<20))

			_, err = f.Seek(0, io.SeekStart)
			require.NoError(t, err)
			var count int
			var last model.Cue
			var readErr error
			growth = heapGrowth(func(sample func()) {
				for c, err := range tt.newReader(f) {
					if err != nil {
						readErr = err
						return
					}
					if count%sampleEvery == 0 {
						sample()
					}
					count++
					last = c
				}
			})
			require.NoError(t, readErr)
			assert.Less(t, growth, uint64(memoryLimit), "read")
			assert.Equal(t, memoryCues, count)
			assert.Equal(t, 399998*time.Second, last.Start)
			assert.Equal(t, "Light event number 199999\n<i>Zone</i> 2", last.Text())
		})
	}
}

func memoryCue(i int) model.Cue {
	return model.Cue{
		Start: time.Duration(i*2) * time.Second,
		End:   time.Duration(i*2)*time.Second + 1500*time.Millisecond,
		Lines: []string{"Light event number " + strconv.Itoa(i), "<i>Zone</i> " + strconv.Itoa(i%7)},
	}
}
