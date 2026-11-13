package mkv_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/container/mkv"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzOpen(f *testing.F) {
	for _, pattern := range []string{"*.mkv", "*.webm"} {
		files, err := filepath.Glob(filepath.Join(testfiles.Path(f, "mkv"), pattern))
		require.NoError(f, err)
		for _, name := range files {
			b, err := os.ReadFile(name)
			require.NoError(f, err)
			f.Add(b)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := mkv.Open(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			requireReadError(t, err)
			return
		}
		for _, track := range file.Tracks() {
			for _, lenient := range []bool{false, true} {
				_, _, err := file.Extract(track.Number, model.CommonReadOptions{Lenient: lenient})
				if err != nil {
					requireReadError(t, err)
				}
			}
		}
	})
}

func requireReadError(t *testing.T, err error) {
	t.Helper()
	require.ErrorIs(t, err, model.ErrParse)
}
