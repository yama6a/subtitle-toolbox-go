package imaging_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/imaging"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func TestEncodePNGThenDecodeGivesTheExactPixels(t *testing.T) {
	t.Parallel()

	nrgba := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	nrgba.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 1})
	nrgba.SetNRGBA(1, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	paletted := image.NewPaletted(image.Rect(5, 5, 7, 6), color.Palette{color.NRGBA{}, color.NRGBA{R: 200, A: 128}})
	paletted.SetColorIndex(6, 5, 1)
	wantPaletted := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	wantPaletted.SetNRGBA(1, 0, color.NRGBA{R: 200, A: 128})

	tests := []struct {
		name string
		img  image.Image
		want *image.NRGBA
	}{
		{"RGBA", nrgba, nrgba},
		{"palette", paletted, wantPaletted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := imaging.EncodePNG(tt.img)
			require.NoError(t, err)
			got, err := imaging.DecodePNG(data)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeChecksTheSizeOfTheImageData(t *testing.T) {
	t.Parallel()

	im, err := imaging.NewImage(image.NewNRGBA(image.Rect(0, 0, 4, 3)), 0, 0, 100, 100)
	require.NoError(t, err)
	im.Width = 5
	_, err = imaging.Decode(im)
	var invalid *model.InvalidArgumentError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "Cannot decode the image: the PNG has 4x3 pixels, but the image data says 5x3.", invalid.Msg)
}

func TestEncodePNGAndNewImageReject(t *testing.T) {
	t.Parallel()

	_, err := imaging.EncodePNG(image.NewNRGBA(image.Rect(0, 0, 0, 5)))
	var invalid *model.InvalidArgumentError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "Cannot encode a PNG of 0x5 pixels: the width and the height must be at least 1.", invalid.Msg)

	_, err = imaging.NewImage(image.NewNRGBA(image.Rect(0, 0, 7681, 1)), 0, 0, 1920, 1080)
	require.ErrorIs(t, err, model.ErrInvalidArgument)
}
