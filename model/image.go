package model

import (
	"slices"
	"strconv"
)

// The limits of an image: 7680 pixels per side and a 3840 x 2160 frame in total.
const (
	MaxImageSide   = 7680
	MaxImagePixels = 3840 * 2160
)

// Image is the PNG bitmap of an image cue and its place on the screen. All sizes are in pixels.
type Image struct {
	// PNG holds the bytes of the PNG file.
	PNG []byte
	// X is the distance of the left edge of the image from the left edge of the screen.
	X int
	// Y is the distance of the top edge of the image from the top edge of the screen.
	Y int
	// Width is the width of the image.
	Width int
	// Height is the height of the image.
	Height int
	// ScreenWidth is the width of the screen that X and Width refer to.
	ScreenWidth int
	// ScreenHeight is the height of the screen that Y and Height refer to.
	ScreenHeight int
}

// Validate checks that every size is at least 1 and that the image fits MaxImageSide and MaxImagePixels.
// It does not decode the PNG. The error matches ErrInvalidArgument.
func (im Image) Validate() error {
	size := strconv.Itoa(im.Width) + "x" + strconv.Itoa(im.Height)
	if im.Width < 1 || im.Height < 1 || im.ScreenWidth < 1 || im.ScreenHeight < 1 {
		return invalidArgument("Cannot create a cue image of " + size + " pixels on a screen of " +
			strconv.Itoa(im.ScreenWidth) + "x" + strconv.Itoa(im.ScreenHeight) + " pixels: every size must be at least 1.")
	}
	if im.Width > MaxImageSide || im.Height > MaxImageSide || im.Width*im.Height > MaxImagePixels {
		return invalidArgument("Cannot create a cue image of " + size + " pixels on a screen of " +
			strconv.Itoa(im.ScreenWidth) + "x" + strconv.Itoa(im.ScreenHeight) + " pixels: an image of " + size +
			" pixels is larger than the limit of " + strconv.Itoa(MaxImageSide) + " pixels per side and " +
			strconv.Itoa(MaxImagePixels) + " pixels in total.")
	}
	return nil
}

// IsImageWithoutText reports whether c holds an image and no lines. Text writers cannot write such a cue.
func (c Cue) IsImageWithoutText() bool {
	return c.Image != nil && len(c.Lines) == 0
}

func cloneImage(im *Image) *Image {
	if im == nil {
		return nil
	}
	c := *im
	c.PNG = slices.Clone(im.PNG)
	return &c
}
