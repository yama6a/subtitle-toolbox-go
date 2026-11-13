package stbox_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// TestCavena890Files loads each real file by its extension, because Cavena 890 has no signature. Each file writes back
// byte for byte, also when the cue format data is gone and the writer encodes the text again.
func TestCavena890Files(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(testfiles.Path(t, "cavena890/real/*.890"))
	require.NoError(t, err)
	require.Len(t, files, 7)
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(file)
			require.NoError(t, err)
			_, found := stbox.Detect(raw)
			assert.False(t, found)

			// A text encoding must not touch the bytes.
			for _, opts := range []stbox.ReadOptions{{}, {Encoding: "ISO-8859-7"}} {
				s, info, err := stbox.Load(file, opts)
				require.NoError(t, err)
				assert.Equal(t, stbox.Cavena890, info.Format)
				assert.Empty(t, info.SourceEncoding)
				assert.Empty(t, info.Warnings)
				assert.True(t, bytes.Equal(raw, writeAs(t, s, stbox.Cavena890)), "round trip with the stored blocks")

				for i, c := range s.All() {
					c.DeleteFormatData("cavena890")
					require.NoError(t, s.SetCue(i, c))
				}
				assert.True(t, bytes.Equal(raw, writeAs(t, s, stbox.Cavena890)), "round trip with encoded text")
			}

			_, _, err = parseBytes(raw, stbox.ReadOptions{})
			assert.ErrorIs(t, err, stbox.ErrUnknownFormat)
		})
	}
}
