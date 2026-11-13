// Package ocr reads the text of image cues with an OCR engine.
package ocr

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Language is a Tesseract model name. Any string works, for example a custom model or "deu+eng".
type Language string

// EngineName names a built-in engine. The engine chooser of package ocr/auto reads it.
type EngineName string

// The names of the built-in engines.
const (
	EngineTesseract EngineName = "tesseract"
	EngineGlyph     EngineName = "glyph"
)

// Engine reads the text of one cue image.
type Engine interface {
	// Recognize returns the text of im. language is a Tesseract model name such as "deu" or "deu+eng",
	// or "" for the engine default. Engines that do not use languages ignore it.
	Recognize(ctx context.Context, im model.Image, language Language) (Text, error)
}

// Text is the result of one image.
type Text struct {
	// Lines holds plain text or core markup, for example "<i>Hello</i>".
	Lines []string
	// Confidence is the confidence of the engine from 0 to 1. Nil means the engine gives none.
	Confidence *float64
}

// Validate checks that Confidence is from 0 to 1. The error matches model.ErrInvalidArgument.
func (t Text) Validate() error {
	if c := t.Confidence; c != nil && !(*c >= 0 && *c <= 1) {
		return &model.InvalidArgumentError{
			Msg: "The confidence must be from 0 to 1, got " + timecode.Number(*c) + ".",
		}
	}
	return nil
}

// CueText is the text that the engine read for one cue.
type CueText struct {
	// CueIndex is the index of the cue in the subtitle.
	CueIndex int
	// Text is the result of the engine, as the engine returned it.
	Text Text
}

// Report lists the cues that the engine read, in cue order.
type Report struct {
	Texts []CueText
}

// ErrRecognize matches every error of an engine that fails on an image.
var ErrRecognize = errors.New("ocr: the engine cannot read the image")

// CueError is an error of the engine on one cue.
type CueError struct {
	// CueIndex is the index of the cue in the subtitle.
	CueIndex int
	// Err is the error of the engine.
	Err error
}

// Error returns the cue index and the cause, for example "ocr: cue 3: the PNG is broken".
func (e *CueError) Error() string {
	return "ocr: cue " + strconv.Itoa(e.CueIndex) + ": " + e.Err.Error()
}

// Unwrap returns the error of the engine.
func (e *CueError) Unwrap() error { return e.Err }

// Is reports whether target is ErrRecognize.
func (e *CueError) Is(target error) bool { return target == ErrRecognize }

// Run sets the lines of every image cue without lines to the text that engine reads, and keeps the images.
// Each cue gets the cleaned lines of the text, without the empty ones. The report holds the text as the engine
// returned it. Run calls the engine for one cue at a time, in cue order, and passes language unchanged.
//
// An error of the engine or of Text.Validate returns a *CueError. Cues read before it keep their new lines,
// and the returned report lists them. A cancelled ctx stops Run before the next cue and returns its error, also when the engine failed because of the cancel.
func Run(ctx context.Context, s *model.Subtitle, engine Engine, language Language) (Report, error) {
	var report Report
	for i := range s.Len() {
		cue, _ := s.Cue(i)
		if !cue.IsImageWithoutText() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return report, fmt.Errorf("ocr: %w", err)
		}
		text, err := engine.Recognize(ctx, *cue.Image, language)
		if err == nil {
			err = text.Validate()
		}
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return report, fmt.Errorf("ocr: %w", ctxErr)
			}
			return report, &CueError{CueIndex: i, Err: err}
		}
		for _, line := range text.Lines {
			cue.AddLine(line)
		}
		if err := s.SetCue(i, cue); err != nil {
			return report, &CueError{CueIndex: i, Err: err}
		}
		report.Texts = append(report.Texts, CueText{CueIndex: i, Text: text})
	}
	return report, nil
}
