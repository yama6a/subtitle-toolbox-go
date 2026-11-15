package mkv_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/container/mkv"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzOpen(f *testing.F) {
	formattest.AddSeeds(f, "mkv/*.mkv", "mkv/*.webm")
	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := mkv.Open(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			requireReadError(t, err)
			return
		}
		for _, track := range file.Tracks() {
			for _, lenient := range []bool{false, true} {
				s, warnings, err := file.Extract(track.Number, model.CommonReadOptions{Lenient: lenient})
				formattest.CheckRead(t, s, warnings, err)
			}
		}
	})
}

func requireReadError(t *testing.T, err error) {
	t.Helper()
	require.ErrorIs(t, err, model.ErrParse)
}
