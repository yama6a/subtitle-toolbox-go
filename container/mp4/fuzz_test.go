package mp4_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v5/container/mp4"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzOpen(f *testing.F) {
	formattest.AddSeeds(f, "mp4/*.mp4", "mp4/*.mov")
	f.Add(single(trackSpec{}, textSample("Hi")))

	f.Fuzz(func(t *testing.T, content []byte) {
		file, err := mp4.Open(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			require.ErrorIs(t, err, model.ErrParse)
			return
		}
		for _, track := range file.Tracks() {
			s, warnings, err := file.Extract(track.Number, model.CommonReadOptions{}, mp4.ExtractOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			for i, c := range s.All() {
				if c.Start < 0 {
					t.Fatalf("cue %d has a negative start", i)
				}
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
