// Package glyph reads cue images with the pure Go OCR engine github.com/yama6a/go-glyph-ocr.
package glyph

import (
	"context"
	"fmt"
	"image"
	"strconv"
	"strings"

	glyphocr "github.com/yama6a/go-glyph-ocr"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v3/markup"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr"
)

// Options holds the settings of the glyph engine. The zero value matches with the subtitle fonts database.
type Options struct {
	// Database holds the glyphs to match. Nil means glyphocr.SubtitleFonts().
	// Engines can share one database, because matching only reads it.
	Database *glyphocr.Database
	// Recognizer holds the options of go-glyph-ocr. They go to glyphocr.NewRecognizer unchanged.
	Recognizer []glyphocr.Option
}

// Engine reads the cue images of one subtitle stream. It is not safe for concurrent use,
// because the recognizer learns the glyph heights of the stream.
type Engine struct {
	recognizer *glyphocr.Recognizer
	database   *glyphocr.Database
}

var _ ocr.Engine = (*Engine)(nil)

// New returns an engine for opts. An invalid recognizer option returns a *model.InvalidArgumentError.
func New(opts Options) (*Engine, error) {
	db := opts.Database
	if db == nil {
		db = glyphocr.SubtitleFonts()
	}
	r, err := glyphocr.NewRecognizer(db, opts.Recognizer...)
	if err != nil {
		return nil, &model.InvalidArgumentError{Option: "glyph.Options.Recognizer", Msg: err.Error()}
	}
	return &Engine{recognizer: r, database: db}, nil
}

// Database returns the database that the engine matches against.
func (e *Engine) Database() *glyphocr.Database { return e.database }

// recognizeError is an error that matches ocr.ErrRecognize.
type recognizeError struct {
	msg string
	err error
}

func (e *recognizeError) Error() string        { return e.msg }
func (e *recognizeError) Unwrap() error        { return e.err }
func (e *recognizeError) Is(target error) bool { return target == ocr.ErrRecognize }

// Recognize returns the text of im. It ignores language, and it returns ctx.Err() before it starts.
// An image that is no PNG returns an error that matches ocr.ErrRecognize.
func (e *Engine) Recognize(ctx context.Context, im model.Image, _ ocr.Language) (ocr.Text, error) {
	if err := ctx.Err(); err != nil {
		return ocr.Text{}, fmt.Errorf("glyph: %w", err)
	}
	pixels, err := bitmap.DecodePNG(im.PNG)
	if err != nil {
		return ocr.Text{}, &recognizeError{
			msg: "The recognizer fails on the cue image at " + strconv.Itoa(im.X) + ", " + strconv.Itoa(im.Y) + ": " + err.Error(),
			err: err,
		}
	}
	return text(e.recognizer.Recognize(nrgba(pixels))), nil
}

func nrgba(p bitmap.Pixels) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, p.Width, p.Height))
	for i, c := range p.RGBA {
		img.Pix[4*i], img.Pix[4*i+1], img.Pix[4*i+2], img.Pix[4*i+3] = byte(c>>24), byte(c>>16), byte(c>>8), byte(c)
	}
	return img
}

func text(r glyphocr.Result) ocr.Text {
	lines := make([]string, len(r.Lines))
	for i, l := range r.Lines {
		lines[i] = line(l)
	}
	confidence := r.Confidence()
	return ocr.Text{Lines: lines, Confidence: &confidence}
}

// line joins the words of l and wraps each run of italic words in one <i> tag.
// A word is italic when more than half of its characters are.
func line(l glyphocr.Line) string {
	var b strings.Builder
	var word strings.Builder
	var chars, italic, words int
	open := false
	flush := func() {
		isItalic := 2*italic > chars
		if open && !isItalic {
			b.WriteString("</i>")
			open = false
		}
		if words > 0 {
			b.WriteByte(' ')
		}
		if isItalic && !open {
			b.WriteString("<i>")
			open = true
		}
		b.WriteString(markup.EscapeText(word.String()))
		word.Reset()
		chars, italic = 0, 0
		words++
	}
	for _, c := range l.Chars {
		if c.Space {
			flush()
			continue
		}
		word.WriteString(c.Text)
		chars++
		if c.Italic {
			italic++
		}
	}
	flush()
	if open {
		b.WriteString("</i>")
	}
	return b.String()
}
