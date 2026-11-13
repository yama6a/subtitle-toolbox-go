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

func withConfidence(cs []glyphocr.Char, values ...float64) []glyphocr.Char {
	for i, v := range values {
		cs[i].Confidence = v
	}
	return cs
}

func TestText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []glyphocr.Line
		want  []string
		conf  float64
	}{
		{
			"italic words share one run and an upright word closes it",
			[]glyphocr.Line{
				{Chars: chars("The wind turns", "II.IIIII.IIIII")},
				{Chars: chars("Snow is here.", "IIII.I...I...")},
				{Chars: chars("one more word", "...I.IIII.III")},
			},
			[]string{"<i>The wind turns</i>", "<i>Snow</i> is here.", "one <i>more word</i>"},
			1,
		},
		{
			"a word is italic above half of its characters",
			[]glyphocr.Line{{Chars: chars("ab cd", "I..II")}},
			[]string{"ab <i>cd</i>"},
			1,
		},
		{
			"2 spaces make an empty word that closes the run",
			[]glyphocr.Line{{Chars: chars("a  b", "I..I")}},
			[]string{"<i>a</i>  <i>b</i>"},
			1,
		},
		{
			"markup characters are escaped",
			[]glyphocr.Line{{Chars: withConfidence(chars("<& b", "...."), 0.5, 1, 1, 0.6)}},
			[]string{"&lt;&amp; b"},
			0.7,
		},
		{"an image without ink gives no lines", nil, []string{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := text(glyphocr.Result{Lines: tt.lines})
			assert.Equal(t, tt.want, got.Lines)
			require.NotNil(t, got.Confidence)
			assert.InDelta(t, tt.conf, *got.Confidence, 1e-9)
		})
	}
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

func TestNewRejectsAnInvalidRecognizerOption(t *testing.T) {
	t.Parallel()

	_, err := New(Options{Recognizer: []glyphocr.Option{glyphocr.InkThreshold(0)}})
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	assert.ErrorContains(t, err, "glyph.Options.Recognizer")
}
