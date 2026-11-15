package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	glyphocr "github.com/yama6a/go-glyph-ocr"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/markup"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr/ocrtest"
)

// ocrDir returns a directory with the CLI fixtures and the PGS files.
func ocrDir(t *testing.T) string {
	t.Helper()

	dir := newDir(t)
	for _, name := range []string{"text_1080p.sup", "text_cyrillic_1080p.sup", "shapes_1080p.sup"} {
		copyFile(t, testfiles.Path(t, "pgs/"+name), filepath.Join(dir, name))
	}
	return dir
}

func TestOCROptionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		want  string
	}{
		{"engine without --ocr", []string{"--ocr-engine", "glyph"}, "Pass --ocr with --ocr-engine."},
		{"language without --ocr", []string{"--ocr-language", "deu"}, "Pass --ocr with --ocr-language."},
		{"database without --ocr", []string{"--ocr-database", "db.nocr"}, "Pass --ocr with --ocr-database."},
		{"unknown engine", []string{"--ocr", "--ocr-engine", "easyocr"}, `The option --ocr-engine must be tesseract or glyph, got "easyocr".`},
		{
			"database with Tesseract",
			[]string{"--ocr", "--ocr-engine", "tesseract", "--ocr-database", "db.nocr"},
			"Pass --ocr-engine glyph with --ocr-database.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := ocrDir(t)
			before := snapshot(t, dir)
			got := call{dir: dir}.run(t, append([]string{"convert", "text_1080p.sup", "--to", "srt", "-o", "out.srt"}, tt.words...)...)
			assert.Equal(t, usageResult("convert", tt.want), got)
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}

func TestOCRConvert(t *testing.T) {
	t.Parallel()

	dir := ocrDir(t)
	empty := filepath.Join(dir, "empty.nocr")
	db, err := glyphocr.NewDatabase()
	require.NoError(t, err)
	require.NoError(t, db.Save(empty))

	golden := string(testfiles.Read(t, "pgs/text_1080p.ocr.srt"))
	got := call{dir: dir}.run(t, "convert", "text_1080p.sup", "--to", "srt", "--ocr", "--ocr-engine", "glyph")
	assert.Equal(t, result{Stdout: golden, Stderr: "text_1080p.sup: OCR 12/12\n"}, got)

	t.Run("an empty database replaces the default one", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "convert", "text_1080p.sup", "--to", "srt", "--ocr", "--ocr-database", "empty.nocr")
		assert.Zero(t, got.Code)
		assert.Contains(t, got.Stdout, "* * *")
		assert.NotContains(t, got.Stdout, "Bergen")
	})
	t.Run("a database that is not gzip exits 3 and writes nothing", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "convert", "text_1080p.sup", "--to", "srt", "--ocr", "--ocr-database", "frames.sub", "-o", "bad.srt")
		assert.Equal(t, exitFile, got.Code)
		assert.Empty(t, got.Stdout)
		assert.True(t, strings.HasPrefix(got.Stderr, "Error: frames.sub: glyphocr: invalid glyph database"), got.Stderr)
		assert.NoFileExists(t, filepath.Join(dir, "bad.srt"))
	})
	t.Run("the output may not overwrite the database file", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "convert", "text_1080p.sup", "--to", "srt", "--ocr", "--ocr-database", "empty.nocr", "-o", "empty.nocr")
		assert.Equal(t, usageResult("convert", "The output empty.nocr "+readsMessage), got)
	})
	t.Run("an interrupt during the Tesseract check exits 130", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		got := call{dir: dir, ctx: ctx}.run(t, "convert", "text_1080p.sup", "--ocr", "--ocr-engine", "tesseract")
		assert.Equal(t, result{Code: exitInterrupt, Stderr: "Stopped by a signal.\n"}, got)
	})
	t.Run("two inputs warn once about the language", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "convert", "text_1080p.sup", "text_cyrillic_1080p.sup", "--to", "srt", "--output-dir", "two",
			"--ocr", "--ocr-engine", "glyph", "--ocr-language", "deu")
		assert.Equal(t, 0, got.Code)
		assert.Equal(t, "Warning: the glyph engine ignores --ocr-language.\n"+
			"text_1080p.sup: OCR 12/12\ntext_cyrillic_1080p.sup: OCR 12/12\n", got.Stderr)
		assert.FileExists(t, filepath.Join(dir, "two", "text_cyrillic_1080p.srt"))
	})
	t.Run("only the forced cues are read", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "convert", "shapes_1080p.sup", "--to", "json", "--forced-only", "--ocr", "--ocr-engine", "glyph")
		assert.Equal(t, 0, got.Code)
		assert.Equal(t, "shapes_1080p.sup: OCR 2/2\n", got.Stderr)
	})
	t.Run("a file without image cues stays as it is", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "convert", "trip.srt", "--ocr", "--ocr-engine", "glyph")
		assert.Equal(t, result{Stdout: convertAs(t, "trip.srt", stbox.SubRip)}, got)
	})
}

func TestOCRProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		total int
		want  string
	}{
		{250, "film.sup: OCR 100/250\nfilm.sup: OCR 200/250\nfilm.sup: OCR 250/250\n"},
		{100, "film.sup: OCR 100/100\n"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.total), func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			p := &progress{engine: &ocrtest.Engine{}, c: &console{stderr: &stderr}, label: "film.sup", total: tt.total}
			for range tt.total {
				_, err := p.Recognize(t.Context(), stbox.Image{}, "")
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, stderr.String())
		})
	}
}

func TestOCRLoadsTheDatabaseOnce(t *testing.T) {
	t.Parallel()

	e := &ocrEdit{engine: ocr.EngineGlyph}
	require.NoError(t, e.load(t.Context(), &console{}))
	databases := make([]*glyphocr.Database, 2)
	for i := range databases {
		engine, err := e.newGlyph(&console{stderr: &bytes.Buffer{}})
		require.NoError(t, err)
		databases[i] = engine.Database()
	}
	assert.Same(t, databases[0], databases[1])
}

// TestConvertOCRAccuracyTesseract reads the fixtures through the CLI and compares each cue with the drawn text.
// The rows skip when the program is missing, unless STBOX_REQUIRE_TESSERACT=1 is set.
func TestConvertOCRAccuracyTesseract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		file  string
		drawn string
		min   float64
	}{
		{"PGS 1080p", "text_1080p.sup", "ocr/drawn/text_1080p.json", 0.99},
		{"PGS 1080p Cyrillic", "text_cyrillic_1080p.sup", "ocr/drawn/text_cyrillic_1080p.json", 0.99},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			words := []string{"convert", tt.file, "--to", "srt", "--ocr", "--ocr-engine", "tesseract"}
			if _, err := exec.LookPath("tesseract"); err != nil {
				if os.Getenv("STBOX_REQUIRE_TESSERACT") == "1" {
					t.Fatal("tesseract is not installed")
				}
				t.Skip("tesseract is not installed")
			}
			if strings.Contains(tt.file, "cyrillic") {
				words = append(words, "--ocr-language", "rus")
			}
			got := call{dir: ocrDir(t)}.run(t, words...)
			require.Zero(t, got.Code, got.Stderr)
			s, _, err := stbox.Parse(strings.NewReader(got.Stdout), int64(len(got.Stdout)), stbox.ReadOptions{})
			require.NoError(t, err)

			var drawn [][]string
			require.NoError(t, json.Unmarshal(testfiles.Read(t, tt.drawn), &drawn))
			require.Equal(t, len(drawn), s.Len())
			var errs, length int
			for i := range s.Len() {
				cue, _ := s.Cue(i)
				want := []rune(plain(drawn[i]))
				errs += testfiles.EditDistance(want, []rune(plain(cue.Lines)))
				length += len(want)
			}
			accuracy := 1 - float64(errs)/float64(length)
			assert.GreaterOrEqual(t, accuracy, tt.min, "%d errors in %d characters", errs, length)
		})
	}
}

func plain(lines []string) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = markup.PlainText(l)
	}
	return strings.Join(out, "\n")
}
