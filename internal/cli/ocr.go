package cli

import (
	"bytes"
	"context"
	"strconv"

	glyphocr "github.com/yama6a/go-glyph-ocr"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr/auto"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr/glyph"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr/tesseract"
)

// progressStep is the number of images between 2 progress lines.
const progressStep = 100

func ocrGroup() group {
	return group{
		name:    "ocr",
		summary: "Read the text of image cues with OCR.",
		options: []option{
			flagOption("ocr", "Read the text of image cues, for example from PGS or VobSub. Use Tesseract when it is installed, "+
				"and the built-in glyph engine when it is not."),
			valueOption("ocr-engine", "ENGINE", "OCR engine for --ocr: tesseract or glyph. Default: tesseract when it is installed."),
			valueOption("ocr-language", "CODE", "Tesseract language for --ocr, for example deu or deu+eng. Default: eng. "+
				"The glyph engine ignores it."),
			valueOption("ocr-database", "FILE", "Glyph database in .nocr format for --ocr. It selects the glyph engine. "+
				"Default: the subtitle fonts database of go-glyph-ocr."),
		},
		build: buildOCR,
	}
}

func buildOCR(a *args, add func(edit)) error {
	if err := needs(a, "ocr", "ocr-engine", "ocr-language", "ocr-database"); err != nil {
		return err
	}
	if !a.Has("ocr") {
		return nil
	}
	name, _, err := a.Choice("ocr-engine", string(ocr.EngineTesseract), string(ocr.EngineGlyph))
	if err != nil {
		return err
	}
	e := &ocrEdit{engine: ocr.EngineName(name)}
	if path, ok := a.Value("ocr-database"); ok {
		e.databasePath, e.hasDatabase = path, true
		if e.engine == ocr.EngineTesseract {
			return usage("Pass --ocr-engine glyph with --ocr-database.")
		}
		e.engine = ocr.EngineGlyph
	}
	language, _ := a.Value("ocr-language")
	e.language = ocr.Language(language)
	e.hasLanguage = a.Has("ocr-language")
	add(e)
	return nil
}

// ocrEdit runs OCR on the image cues without lines. load chooses the engine and reads the database once per run.
type ocrEdit struct {
	// engine is the engine that the options ask for, "" for the default.
	engine       ocr.EngineName
	language     ocr.Language
	hasLanguage  bool
	databasePath string
	hasDatabase  bool

	// load sets tesseract for the Tesseract engine, and database for the glyph engine.
	tesseract *tesseract.Engine
	database  *glyphocr.Database
	warned    bool
}

var _ edit = &ocrEdit{}

// load picks the engine and checks it, before any input is read. A missing Tesseract or a missing language
// is a usage error. A database that go-glyph-ocr rejects is a *fileError.
func (e *ocrEdit) load(ctx context.Context, c *console) error {
	chosen, err := auto.Choose(ctx, auto.Options{Engine: e.engine})
	if err != nil {
		return checkError(ctx, err)
	}
	if chosen == ocr.EngineTesseract {
		if e.tesseract, err = tesseract.New(tesseract.Options{Language: e.language}); err != nil {
			return usage(message(err))
		}
		if err := e.tesseract.RequireLanguages(ctx, ""); err != nil {
			return checkError(ctx, err)
		}
		return nil
	}
	if !e.hasDatabase {
		e.database = glyphocr.SubtitleFonts()
		return nil
	}
	content, err := readSideText(c, e.databasePath)
	if err != nil {
		return err
	}
	if e.database, err = glyphocr.ReadDatabase(bytes.NewReader(content)); err != nil {
		return &fileError{msg: e.databasePath + ": " + err.Error()}
	}
	return nil
}

// checkError returns the error of ctx unchanged, so an interrupt gives exit code 130. Any other error is a usage error.
func checkError(ctx context.Context, err error) error {
	if isInterrupt(ctx, err) {
		return err
	}
	return usage(message(err))
}

func (e *ocrEdit) apply(ctx context.Context, s *stbox.Subtitle, c *console, label string) (*stbox.Subtitle, error) {
	var total int
	for i := range s.Len() {
		if cue, _ := s.Cue(i); cue.IsImageWithoutText() {
			total++
		}
	}
	if total == 0 {
		return s, nil
	}
	var engine ocr.Engine = e.tesseract
	if e.database != nil {
		var err error
		if engine, err = e.newGlyph(c); err != nil {
			return nil, err
		}
	}
	_, err := ocr.Run(ctx, s, &progress{engine: engine, c: c, label: label, total: total}, "")
	return s, err
}

// newGlyph returns the glyph engine of one input. It is new for each input, because its recognizer learns
// the glyph heights of one stream. All glyph engines share one database.
func (e *ocrEdit) newGlyph(c *console) (*glyph.Engine, error) {
	if e.hasLanguage && !e.warned {
		c.err("Warning: the glyph engine ignores --ocr-language.\n")
		e.warned = true
	}
	return glyph.New(glyph.Options{Database: e.database})
}

// progress prints the progress of one input to standard error.
type progress struct {
	engine      ocr.Engine
	c           *console
	label       string
	total, done int
}

var _ ocr.Engine = &progress{}

func (p *progress) Recognize(ctx context.Context, im stbox.Image, language ocr.Language) (ocr.Text, error) {
	text, err := p.engine.Recognize(ctx, im, language)
	if err != nil {
		return text, err
	}
	p.done++
	if p.done%progressStep == 0 || p.done == p.total {
		p.c.err(p.label + ": OCR " + strconv.Itoa(p.done) + "/" + strconv.Itoa(p.total) + "\n")
	}
	return text, nil
}
