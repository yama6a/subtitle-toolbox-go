package glyph

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	glyphocr "github.com/yama6a/go-glyph-ocr"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr"
)

// chars builds the characters of text. A rune marks an italic character where pattern holds "I" at its index.
func chars(text, pattern string) []glyphocr.Char {
	out := make([]glyphocr.Char, 0, len(text))
	for i, r := range []rune(text) {
		out = append(out, glyphocr.Char{Text: string(r), Confidence: 1, Italic: pattern[i] == 'I', Space: r == ' '})
	}
	return out
}

func TestText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []glyphocr.Line
		want  []string
	}{
		{
			"italic words share one run and an upright word closes it",
			[]glyphocr.Line{
				{Chars: chars("The wind turns", "II.IIIII.IIIII")},
				{Chars: chars("Snow is here.", "IIII.I...I...")},
				{Chars: chars("one more word", "...I.IIII.III")},
			},
			[]string{"<i>The wind turns</i>", "<i>Snow</i> is here.", "one <i>more word</i>"},
		},
		{
			"a word is italic above half of its characters",
			[]glyphocr.Line{{Chars: chars("ab cd", "I..II")}},
			[]string{"ab <i>cd</i>"},
		},
		{
			"2 spaces make an empty word that closes the run",
			[]glyphocr.Line{{Chars: chars("a  b", "I..I")}},
			[]string{"<i>a</i>  <i>b</i>"},
		},
		{
			"markup characters are escaped",
			[]glyphocr.Line{{Chars: chars("<& b", "....")}},
			[]string{"&lt;&amp; b"},
		},
		{"an image without ink gives no lines", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, text(glyphocr.Result{Lines: tt.lines}).Lines)
		})
	}
}

func TestTextConfidenceIsTheMeanWithoutSpaces(t *testing.T) {
	t.Parallel()

	line := chars("< b", "...")
	line[0].Confidence, line[2].Confidence = 0.5, 0.9
	got := text(glyphocr.Result{Lines: []glyphocr.Line{{Chars: line}}})
	require.NotNil(t, got.Confidence)
	assert.InDelta(t, 0.7, *got.Confidence, 1e-9)

	empty := text(glyphocr.Result{})
	require.NotNil(t, empty.Confidence)
	assert.Zero(t, *empty.Confidence)
}

func TestRecognizeErrors(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	var blank bytes.Buffer
	require.NoError(t, png.Encode(&blank, image.NewNRGBA(image.Rect(0, 0, 4, 4))))

	tests := []struct {
		name    string
		ctx     context.Context //nolint:containedctx // the row holds the context of its call
		png     []byte
		wantErr string
		wantIs  error
		lines   int
	}{
		{"no PNG", t.Context(), []byte("no png"), "The recognizer fails on the cue image at 3, 4: ", ocr.ErrRecognize, 0},
		{"cancelled context", cancelled, blank.Bytes(), "context canceled", context.Canceled, 0},
		{"transparent image", t.Context(), blank.Bytes(), "", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, err := New(Options{Database: glyphocr.Latin()})
			require.NoError(t, err)
			got, err := e.Recognize(tt.ctx, model.Image{PNG: tt.png, X: 3, Y: 4, Width: 4, Height: 4}, "deu")
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Len(t, got.Lines, tt.lines)
				return
			}
			require.ErrorIs(t, err, tt.wantIs)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNewPassesTheDatabaseAndTheOptions(t *testing.T) {
	t.Parallel()

	db := glyphocr.Latin()
	e, err := New(Options{Database: db})
	require.NoError(t, err)
	assert.Same(t, db, e.Database())

	_, err = New(Options{Recognizer: []glyphocr.Option{glyphocr.InkThreshold(0)}})
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	assert.ErrorContains(t, err, "glyph.Options.Recognizer")
}
