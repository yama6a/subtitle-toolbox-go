// Package imaging decodes and builds the bitmaps of image cues.
package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// DecodePNG returns the pixels of data with straight alpha. It checks the size limit before it inflates the data.
// The error is an *model.InvalidArgumentError.
func DecodePNG(data []byte) (*image.NRGBA, error) {
	p, err := bitmap.DecodePNG(data)
	if err != nil {
		return nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, p.Width, p.Height))
	for i, c := range p.RGBA {
		img.Pix[4*i], img.Pix[4*i+1], img.Pix[4*i+2], img.Pix[4*i+3] = byte(c>>24), byte(c>>16), byte(c>>8), byte(c)
	}
	return img, nil
}

// Decode returns the pixels of the image of a cue. The PNG size must equal im.Width x im.Height.
func Decode(im model.Image) (*image.NRGBA, error) {
	img, err := DecodePNG(im.PNG)
	if err != nil {
		return nil, err
	}
	if b := img.Bounds(); b.Dx() != im.Width || b.Dy() != im.Height {
		return nil, &model.InvalidArgumentError{Msg: "Cannot decode the image: the PNG has " + strconv.Itoa(b.Dx()) + "x" + strconv.Itoa(b.Dy()) +
			" pixels, but the image data says " + strconv.Itoa(im.Width) + "x" + strconv.Itoa(im.Height) + "."}
	}
	return img, nil
}

// EncodePNG writes img as a PNG: a palette PNG for *image.Paletted, else 8-bit RGBA.
// The same pixels give the same bytes.
func EncodePNG(img image.Image) ([]byte, error) {
	b := img.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 {
		return nil, &model.InvalidArgumentError{Msg: "Cannot encode a PNG of " + strconv.Itoa(b.Dx()) + "x" + strconv.Itoa(b.Dy()) +
			" pixels: the width and the height must be at least 1."}
	}
	if _, ok := img.(*image.Paletted); !ok {
		img = nrgbaOf(img)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, &model.InvalidArgumentError{Msg: "Cannot encode a PNG: " + err.Error() + "."}
	}
	return buf.Bytes(), nil
}

// NewImage encodes img and places it at x, y on a screen of screenWidth x screenHeight pixels.
// The error is an *model.InvalidArgumentError when img or the sizes break the limits of model.Image.
func NewImage(img image.Image, x, y, screenWidth, screenHeight int) (model.Image, error) {
	b := img.Bounds()
	im := model.Image{X: x, Y: y, Width: b.Dx(), Height: b.Dy(), ScreenWidth: screenWidth, ScreenHeight: screenHeight}
	if err := im.Validate(); err != nil {
		return model.Image{}, err
	}
	data, err := EncodePNG(img)
	if err != nil {
		return model.Image{}, err
	}
	im.PNG = data
	return im, nil
}

// nrgbaOf copies img to straight alpha. draw.Draw would go through premultiplied values and change pixels that are not opaque.
func nrgbaOf(img image.Image) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	if n, ok := img.(*image.NRGBA64); ok {
		for y := range b.Dy() {
			for x := range b.Dx() {
				p := n.Pix[(b.Min.Y+y-n.Rect.Min.Y)*n.Stride+(b.Min.X+x-n.Rect.Min.X)*8:]
				out.SetNRGBA(x, y, color.NRGBA{R: p[0], G: p[2], B: p[4], A: p[6]})
			}
		}
		return out
	}
	for y := range b.Dy() {
		for x := range b.Dx() {
			c, _ := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			out.SetNRGBA(x, y, c)
		}
	}
	return out
}
