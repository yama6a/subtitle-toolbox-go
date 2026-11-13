package tesseract

import (
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr"
)

// Options holds the settings of the Tesseract engine. The zero value uses the defaults.
type Options struct {
	// Language is the model for cues where Recognize gets no language, for example "deu" or "deu+eng". "" means "eng".
	Language ocr.Language
	// PageSegmentationMode is the --psm value from 0 to 13. Nil means 6, one block of text.
	PageSegmentationMode *int
	// Program is the path or the name of the tesseract program, looked up in PATH. "" means "tesseract".
	Program string
	// Scale is the factor that scales the image up before OCR, from 1 to 8. 0 means 2 on screens below 720 lines, else 1.
	Scale float64
	// NoInvert keeps the grey levels as they are. By default light text becomes dark on white.
	NoInvert bool
	// Threshold turns grey levels below it black and the others white, from 1 to 255. 0 keeps the grey levels.
	Threshold int
}

// Validate checks the ranges. The error is a *model.InvalidArgumentError that names the option.
func (o Options) Validate() error {
	if m := o.PageSegmentationMode; m != nil && (*m < 0 || *m > 13) {
		return invalid("PageSegmentationMode", "The page segmentation mode must be from 0 to 13, got "+strconv.Itoa(*m)+".")
	}
	if o.Scale != 0 && !(o.Scale >= 1 && o.Scale <= 8) {
		return invalid("Scale", "The scale must be from 1 to 8, got "+timecode.Number(o.Scale)+".")
	}
	if o.Threshold != 0 && (o.Threshold < 1 || o.Threshold > 255) {
		return invalid("Threshold", "The threshold must be from 1 to 255, got "+strconv.Itoa(o.Threshold)+".")
	}
	return nil
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "tesseract.Options." + field, Msg: msg}
}
