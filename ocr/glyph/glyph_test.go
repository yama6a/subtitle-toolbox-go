package glyph_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	glyphocr "github.com/yama6a/go-glyph-ocr"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/fixing"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/markup"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr/glyph"
)

func TestGoldens(t *testing.T) {
	t.Parallel()

	latin := glyph.Options{
		Database:   glyphocr.Latin(),
		Recognizer: []glyphocr.Option{glyphocr.NoLineContext()},
	}
	tests := []struct {
		name   string
		file   string
		golden string
		drawn  string
		opts   glyph.Options
		min    float64
	}{
		{"PGS 1080p, 44 to 60 px", "pgs/text_1080p.sup", "pgs/text_1080p.ocr.srt", "ocr/drawn/text_1080p.json", glyph.Options{}, 1},
		{"VobSub 576p, 24 to 30 px", "vobsub/text-pal.idx", "vobsub/text-pal.ocr.srt", "ocr/drawn/text-pal.json", glyph.Options{}, 0.97},
		{"PGS 1080p, Latin database", "pgs/text_1080p.sup", "fixing/text_1080p.ocr.srt", "", latin, 0},
		{"VobSub 576p, Latin database", "vobsub/text-pal.idx", "fixing/text-pal.ocr.srt", "", latin, 0},
		{"fixing ocr-en, Latin database", "fixing/ocr-en.sup", "fixing/ocr-en.ocr.srt", "", latin, 0},
		{"fixing ocr-de, Latin database", "fixing/ocr-de.sup", "fixing/ocr-de.ocr.srt", "", latin, 0},
		{"fixing ocr-fr, Latin database", "fixing/ocr-fr.sup", "fixing/ocr-fr.ocr.srt", "", latin, 0},
		{"fixing ocr-es, Latin database", "fixing/ocr-es.sup", "fixing/ocr-es.ocr.srt", "", latin, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := stbox.Load(testfiles.Path(t, tt.file), stbox.ReadOptions{})
			require.NoError(t, err)
			e, err := glyph.New(tt.opts)
			require.NoError(t, err)
			_, err = ocr.Run(t.Context(), s, e, "")
			require.NoError(t, err)

			var out bytes.Buffer
			_, err = stbox.Write(&out, s, stbox.SubRip, stbox.WriteOptions{})
			require.NoError(t, err)
			assert.Equal(t, string(testfiles.Read(t, tt.golden)), out.String())

			if tt.drawn == "" {
				return
			}
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
	var out []byte
	for i, l := range lines {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, markup.PlainText(l)...)
	}
	return string(out)
}

func TestRecognizerOptionsReachTheRecognizer(t *testing.T) {
	t.Parallel()

	s, _, err := stbox.Load(testfiles.Path(t, "pgs/text_1080p.sup"), stbox.ReadOptions{})
	require.NoError(t, err)
	cue, _ := s.Cue(0)

	empty, err := glyphocr.NewDatabase()
	require.NoError(t, err)
	e, err := glyph.New(glyph.Options{Database: empty, Recognizer: []glyphocr.Option{glyphocr.UnknownText("#")}})
	require.NoError(t, err)
	got, err := e.Recognize(context.Background(), *cue.Image, "")
	require.NoError(t, err)

	require.Len(t, got.Lines, 1)
	assert.Regexp(t, `^#+( #+)+$`, got.Lines[0])
	assert.Zero(t, *got.Confidence)
}

func TestFixingTheOCRTextKeepsTheImages(t *testing.T) {
	t.Parallel()

	s, _, err := stbox.Load(testfiles.Path(t, "fixing/ocr-fr.sup"), stbox.ReadOptions{})
	require.NoError(t, err)
	report, err := fixing.Apply(s, fixing.Options{Language: "fr"})
	require.NoError(t, err)
	assert.Empty(t, report.Fixes)

	e, err := glyph.New(glyph.Options{
		Database:   glyphocr.Latin(),
		Recognizer: []glyphocr.Option{glyphocr.NoLineContext()},
	})
	require.NoError(t, err)
	_, err = ocr.Run(t.Context(), s, e, "")
	require.NoError(t, err)
	report, err = fixing.Apply(s, fixing.Options{Language: "fr"})
	require.NoError(t, err)

	require.NotEmpty(t, report.Fixes)
	assert.Equal(t, "ll pleut. lls restent à la maison.", report.Fixes[0].Before)
	assert.Equal(t, "Il pleut. Ils restent à la maison.", report.Fixes[0].After)
	cue, _ := s.Cue(0)
	assert.NotNil(t, cue.Image)
}
