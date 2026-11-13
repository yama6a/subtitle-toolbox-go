// Package auto picks and builds an OCR engine.
package auto

import (
	"context"
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr/glyph"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr/tesseract"
)

// Options holds the settings of Choose and New.
type Options struct {
	// Engine names the engine. "" picks Tesseract when it is installed, else glyph.
	Engine ocr.EngineName
	// TesseractProgram is the path or the name of the tesseract program. "" means "tesseract".
	TesseractProgram string
}

// Choose returns the engine that opts select. An explicit engine wins. The glyph engine is always available.
// A missing Tesseract that opts ask for returns the *model.InvalidArgumentError of tesseract.RequireInstalled.
// An unknown engine name returns a *model.InvalidArgumentError too.
func Choose(ctx context.Context, opts Options) (ocr.EngineName, error) {
	switch opts.Engine {
	case ocr.EngineGlyph:
		return ocr.EngineGlyph, nil
	case ocr.EngineTesseract:
		if err := tesseract.RequireInstalled(ctx, opts.TesseractProgram); err != nil {
			return "", err
		}
		return ocr.EngineTesseract, nil
	case "":
		if tesseract.Installed(ctx, opts.TesseractProgram) {
			return ocr.EngineTesseract, nil
		}
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("auto: %w", err)
		}
		return ocr.EngineGlyph, nil
	default:
		return "", &model.InvalidArgumentError{Option: "auto.Options.Engine", Msg: "The OCR engine must be tesseract or glyph, got \"" + string(opts.Engine) + "\"."}
	}
}

// New returns the engine that Choose picks, with its default options.
func New(ctx context.Context, opts Options) (ocr.Engine, error) { //nolint:ireturn // the chosen engine depends on opts
	name, err := Choose(ctx, opts)
	if err != nil {
		return nil, err
	}
	if name == ocr.EngineTesseract {
		return tesseract.New(tesseract.Options{Program: opts.TesseractProgram})
	}
	return glyph.New(glyph.Options{})
}
