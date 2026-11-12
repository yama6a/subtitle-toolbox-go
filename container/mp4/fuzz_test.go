package mp4_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/container/mp4"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const timeLimit = 100000 * time.Hour

func FuzzOpen(f *testing.F) {
	matches, err := filepath.Glob(filepath.Join(testfiles.Path(f, "mp4"), "*.mp4"))
	require.NoError(f, err)
	movs, err := filepath.Glob(filepath.Join(testfiles.Path(f, "mp4"), "*.mov"))
	require.NoError(f, err)
	for _, name := range append(matches, movs...) {
		content, err := os.ReadFile(name)
		require.NoError(f, err)
		f.Add(content)
	}
	f.Add(single(trackSpec{}, textSample("Hi")))

	f.Fuzz(func(t *testing.T, content []byte) {
		file, err := mp4.Open(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			require.ErrorIs(t, err, model.ErrParse)
			return
		}
		for _, track := range file.Tracks() {
			s, _, err := file.Extract(track.Number, model.CommonReadOptions{}, mp4.ExtractOptions{})
			if err != nil {
				require.ErrorIs(t, err, model.ErrParse)
				continue
			}
			var previous time.Duration
			for _, c := range s.Cues() {
				require.GreaterOrEqual(t, c.Start, previous)
				previous = c.Start
				require.Zero(t, c.Start%time.Millisecond)
				require.Zero(t, c.End%time.Millisecond)
				require.Less(t, c.End, timeLimit)
			}
			var out bytes.Buffer
			var werr error
			switch track.Format { //nolint:exhaustive // only these 3 formats come from MP4 tracks
			case model.SubRip:
				_, werr = srt.Write(&out, s, model.CommonWriteOptions{}, srt.WriteOptions{})
			case model.SCC:
				_, werr = scc.Write(&out, s, model.CommonWriteOptions{}, scc.WriteOptions{})
			case model.MCC:
				_, werr = mcc.Write(&out, s, model.CommonWriteOptions{}, mcc.WriteOptions{})
			}
			if werr != nil {
				require.ErrorIs(t, werr, model.ErrUnwritable)
			}
		}
	})
}
