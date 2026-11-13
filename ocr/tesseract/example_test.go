package tesseract_test

import (
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/v3/ocr/tesseract"
)

func ExampleNew() {
	_, err := tesseract.New(tesseract.Options{Scale: 0.5})
	fmt.Println(err)
	// Output:
	// stbox: invalid argument: tesseract.Options.Scale: The scale must be from 1 to 8, got 0.5.
}
