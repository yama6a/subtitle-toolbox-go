// Package glyph reads cue images with the pure Go OCR engine github.com/yama6a/go-glyph-ocr.
package glyph

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	glyphocr "github.com/yama6a/go-glyph-ocr"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v4/markup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr"
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
	return text(e.recognizer.Recognize(pixels.NRGBA())), nil
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
	open := false
	for i, w := range words(l) {
		italic := 2*italicCount(w) > len(w)
		if open && !italic {
			b.WriteString("</i>")
			open = false
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		if italic && !open {
			b.WriteString("<i>")
			open = true
		}
		for _, c := range w {
			b.WriteString(markup.EscapeText(c.Text))
		}
	}
	if open {
		b.WriteString("</i>")
	}
	return b.String()
}

// words splits the characters of l at the spaces. 2 spaces in a row give an empty word.
func words(l glyphocr.Line) [][]glyphocr.Char {
	var out [][]glyphocr.Char
	var word []glyphocr.Char
	for _, c := range l.Chars {
		if c.Space {
			out = append(out, word)
			word = nil
			continue
		}
		word = append(word, c)
	}
	return append(out, word)
}

func italicCount(word []glyphocr.Char) int {
	n := 0
	for _, c := range word {
		if c.Italic {
			n++
		}
	}
	return n
}
