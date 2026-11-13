package stbox_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/imaging"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// TestVobSubToPGS writes each VobSub track as PGS and compares the file with its golden.
// Reading the result gives the same cues. Limited range YCrCb has no code for some RGB colours,
// so a colour channel can differ by 1. The alpha does not change.
func TestVobSubToPGS(t *testing.T) {
	t.Parallel()

	for _, g := range vobsubImageGoldens {
		t.Run(g.golden, func(t *testing.T) {
			t.Parallel()

			s, _, err := stbox.Load(testfiles.Path(t, g.fixture), g.options)
			require.NoError(t, err)
			var out bytes.Buffer
			_, err = stbox.Write(&out, s, stbox.PGS, stbox.WriteOptions{})
			require.NoError(t, err)
			golden.Equal(t, testfiles.Path(t, g.golden+".to.sup"), out.Bytes(), *update)

			back, _, err := stbox.Parse(bytes.NewReader(out.Bytes()), int64(out.Len()), stbox.ReadOptions{Format: stbox.PGS})
			require.NoError(t, err)
			require.Equal(t, s.Len(), back.Len())
			for i, c := range s.Cues() {
				d, _ := back.Cue(i)
				assert.Equal(t, c.Start, d.Start, "start of cue %d", i)
				assert.Equal(t, c.End, d.End, "end of cue %d", i)
				assert.Equal(t, [6]int{c.Image.X, c.Image.Y, c.Image.Width, c.Image.Height, c.Image.ScreenWidth, c.Image.ScreenHeight},
					[6]int{d.Image.X, d.Image.Y, d.Image.Width, d.Image.Height, d.Image.ScreenWidth, d.Image.ScreenHeight}, "place of cue %d", i)
				want, err := imaging.Decode(*c.Image)
				require.NoError(t, err)
				got, err := imaging.Decode(*d.Image)
				require.NoError(t, err)
				for p := range want.Pix {
					delta := 1.0
					if p%4 == 3 {
						delta = 0
					}
					assert.InDelta(t, int(want.Pix[p]), int(got.Pix[p]), delta, "cue %d byte %d", i, p)
				}
			}
		})
	}
}
