package bitmap_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func FuzzDecodePNG(f *testing.F) {
	f.Add([]byte("not a PNG"))
	f.Add(pngSpec{width: 2, height: 1, depth: 8, colorType: rgbaType, rows: [][]byte{{1, 2, 3, 4, 5, 6, 7, 8}}}.bytes())
	f.Add(pngSpec{width: 2, height: 1, depth: 1, colorType: paletteType, palette: []byte{1, 2, 3}, trns: []byte{9}, rows: [][]byte{{0x40}}}.bytes())
	f.Add(pngSpec{width: 1, height: 1, depth: 16, colorType: grayAlpha, rows: [][]byte{{1, 2, 3, 4}}}.bytes())
	f.Add(pngSpec{width: 65535, height: 65535, depth: 8, colorType: rgbaType}.bytes()[:33])
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := bitmap.DecodePNG(data)
		if err != nil {
			var invalid *model.InvalidArgumentError
			require.ErrorAs(t, err, &invalid)
			return
		}
		require.LessOrEqual(t, p.Width, model.MaxImageSide)
		require.LessOrEqual(t, p.Width*p.Height, model.MaxImagePixels)
		require.Len(t, p.RGBA, p.Width*p.Height)
		_, _, err = bitmap.DecodeIndexedPNG(data)
		require.NoError(t, err)
	})
}
