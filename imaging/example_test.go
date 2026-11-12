package imaging_test

import (
	"fmt"
	"image"
	"image/color"

	"github.com/yama6a/subtitle-toolbox-go/v3/imaging"
)

func ExampleNewImage() {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	im, err := imaging.NewImage(img, 100, 900, 1920, 1080)
	fmt.Println(im.X, im.Y, im.Width, im.Height, im.ScreenWidth, im.ScreenHeight, err)
	// Output: 100 900 4 2 1920 1080 <nil>
}

func ExampleEncodePNG() {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	data, err := imaging.EncodePNG(img)
	fmt.Println(string(data[1:4]), err)
	// Output: PNG <nil>
}

func ExampleDecodePNG() {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 128, B: 1, A: 64})
	data, _ := imaging.EncodePNG(img)
	got, err := imaging.DecodePNG(data)
	fmt.Println(got.NRGBAAt(0, 0), err)
	// Output: {255 128 1 64} <nil>
}

func ExampleDecode() {
	im, _ := imaging.NewImage(image.NewNRGBA(image.Rect(0, 0, 3, 2)), 0, 0, 720, 576)
	got, err := imaging.Decode(im)
	fmt.Println(got.Bounds().Dx(), got.Bounds().Dy(), err)
	// Output: 3 2 <nil>
}
