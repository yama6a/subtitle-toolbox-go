// Package ocrtest holds a fake OCR engine for tests.
package ocrtest

import (
	"context"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr"
)

// Call is one call of Engine.Recognize.
type Call struct {
	Image    model.Image
	Language ocr.Language
}

// Engine is an ocr.Engine that returns the same text for every image and records each call.
// It is not safe for concurrent use.
type Engine struct {
	// Text is the result of every call.
	Text ocr.Text
	// Calls lists the calls in order.
	Calls []Call
}

var _ ocr.Engine = (*Engine)(nil)

// Recognize records the call and returns e.Text.
func (e *Engine) Recognize(_ context.Context, im model.Image, language ocr.Language) (ocr.Text, error) {
	e.Calls = append(e.Calls, Call{Image: im, Language: language})
	return e.Text, nil
}
