// Package tesseract reads cue images with the tesseract program.
package tesseract

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr"
)

// Engine reads cue images with the tesseract program. It is safe for concurrent use.
type Engine struct {
	program  string
	language ocr.Language
	mode     string
	scale    float64
	noInvert bool
	thresh   int
}

var _ ocr.Engine = (*Engine)(nil)

// New returns an engine for opts after it checks them.
func New(opts Options) (*Engine, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	e := &Engine{
		program:  opts.Program,
		language: opts.Language,
		mode:     "6",
		scale:    opts.Scale,
		noInvert: opts.NoInvert,
		thresh:   opts.Threshold,
	}
	if e.program == "" {
		e.program = "tesseract"
	}
	if e.language == "" {
		e.language = ocr.English
	}
	if opts.PageSegmentationMode != nil {
		e.mode = strconv.Itoa(*opts.PageSegmentationMode)
	}
	return e, nil
}

// recognizeError is an error that matches ocr.ErrRecognize.
type recognizeError struct {
	msg string
	err error
}

func (e *recognizeError) Error() string        { return e.msg }
func (e *recognizeError) Unwrap() error        { return e.err }
func (e *recognizeError) Is(target error) bool { return target == ocr.ErrRecognize }

// Recognize returns the text of im. A non-empty language wins over Options.Language.
// It checks the program and the language data first, so a missing model fails before any image is written.
func (e *Engine) Recognize(ctx context.Context, im model.Image, language ocr.Language) (ocr.Text, error) {
	if language == "" {
		language = e.language
	}
	if err := e.RequireLanguages(ctx, language); err != nil {
		return ocr.Text{}, err
	}
	at := strconv.Itoa(im.X) + ", " + strconv.Itoa(im.Y)

	pixels, err := bitmap.DecodePNG(im.PNG)
	if err != nil {
		return ocr.Text{}, &recognizeError{msg: "Cannot read the cue image at " + at + ": " + err.Error(), err: err}
	}
	scale := e.scale
	if scale == 0 {
		scale = 1
		if im.ScreenHeight < 720 {
			scale = 2
		}
	}

	file, err := writeTemp(pgm(pixels, scale, e.noInvert, e.thresh))
	if err != nil {
		return ocr.Text{}, err
	}
	defer os.Remove(file)

	stdout, stderr, exit, err := run(ctx, e.program, file, "stdout", "-l", string(language), "--psm", e.mode, "-c", "tessedit_create_tsv=1")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ocr.Text{}, fmt.Errorf("tesseract: %w", ctxErr)
		}
		return ocr.Text{}, &recognizeError{msg: "Cannot start Tesseract for the cue image at " + at + ": " + err.Error(), err: err}
	}
	if exit != 0 {
		return ocr.Text{}, &recognizeError{msg: "Tesseract exits with code " + strconv.Itoa(exit) + " for the cue image at " + at + ": " + strings.TrimSpace(stderr)}
	}
	lines, confidence := parseTSV(stdout)
	return ocr.Text{Lines: lines, Confidence: confidence}, nil
}

// writeTemp writes data to a new temporary file and returns its name. The caller removes the file.
func writeTemp(data []byte) (string, error) {
	f, err := os.CreateTemp("", "stbox-ocr-*.pgm")
	if err != nil {
		return "", fmt.Errorf("tesseract: %w", err)
	}
	_, err = f.Write(data)
	if err := errors.Join(err, f.Close()); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("tesseract: %w", err)
	}
	return f.Name(), nil
}
