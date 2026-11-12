package ocr_test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v3/ocr/ocrtest"
)

// subtitleWithImages returns 4 cues: an image cue without lines, an image cue with lines,
// a text cue and a second image cue without lines.
func subtitleWithImages(t *testing.T) *model.Subtitle {
	t.Helper()

	r := bytes.NewReader(testfiles.Read(t, "json/real/own_image_cues.json"))
	s, _, err := stbox.Parse(r, r.Size(), stbox.ReadOptions{})
	require.NoError(t, err)
	first, _ := s.Cue(0)
	last := first.Clone()
	last.Start, last.End = 6*time.Second, 7*time.Second
	last.Image.X = 40
	s.AddCue(model.Cue{Start: 5 * time.Second, End: 5500 * time.Millisecond, Lines: []string{"Text cue"}})
	s.AddCue(last)
	return s
}

type failingEngine struct {
	calls  int
	failAt int
}

func (e *failingEngine) Recognize(context.Context, model.Image, ocr.Language) (ocr.Text, error) {
	e.calls++
	if e.calls == e.failAt {
		return ocr.Text{}, errors.New("the PNG is broken")
	}
	return ocr.Text{Lines: []string{"Read"}}, nil
}

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      ocr.Text
		language  ocr.Language
		wantLines []string
	}{
		{name: "markup lines", text: ocr.Text{Lines: []string{"<i>Line one</i>", "Line two"}}, wantLines: []string{"<i>Line one</i>", "Line two"}},
		{name: "empty and padded lines", text: ocr.Text{Lines: []string{" Line one ", "", "Line  two"}}, wantLines: []string{"Line one", "Line two"}},
		{name: "no lines", text: ocr.Text{}, wantLines: nil},
		{name: "language constant", text: ocr.Text{Lines: []string{"Hallo"}}, language: ocr.German, wantLines: []string{"Hallo"}},
		{name: "custom language", text: ocr.Text{Lines: []string{"Hallo"}}, language: "deu+my_model", wantLines: []string{"Hallo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleWithImages(t)
			engine := &ocrtest.Engine{Text: tt.text}

			report, err := ocr.Run(t.Context(), s, engine, tt.language)

			require.NoError(t, err)
			assert.Equal(t, []ocr.CueText{{CueIndex: 0, Text: tt.text}, {CueIndex: 3, Text: tt.text}}, report.Texts)
			require.Len(t, engine.Calls, 2)
			assert.Equal(t, tt.language, engine.Calls[0].Language)
			assert.Equal(t, 40, engine.Calls[1].Image.X)
			for i, wantLines := range [][]string{tt.wantLines, {"Platform four"}, {"Text cue"}, tt.wantLines} {
				c, _ := s.Cue(i)
				assert.Equal(t, wantLines, c.Lines, "cue %d", i)
				assert.Equal(t, i != 2, c.Image != nil, "cue %d", i)
			}
		})
	}
}

func TestRunWithoutImageCuesDoesNotCallTheEngine(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"Text"}})
	engine := &ocrtest.Engine{}

	report, err := ocr.Run(t.Context(), s, engine, "")

	require.NoError(t, err)
	assert.Empty(t, report.Texts)
	assert.Empty(t, engine.Calls)
}

func TestRunStopsOnAnEngineError(t *testing.T) {
	t.Parallel()

	s := subtitleWithImages(t)

	report, err := ocr.Run(t.Context(), s, &failingEngine{failAt: 2}, "")

	var cueErr *ocr.CueError
	require.ErrorAs(t, err, &cueErr)
	require.ErrorIs(t, err, ocr.ErrRecognize)
	assert.Equal(t, 3, cueErr.CueIndex)
	require.EqualError(t, err, "ocr: cue 3: the PNG is broken")
	assert.Equal(t, []ocr.CueText{{CueIndex: 0, Text: ocr.Text{Lines: []string{"Read"}}}}, report.Texts)
	first, _ := s.Cue(0)
	last, _ := s.Cue(3)
	assert.Equal(t, []string{"Read"}, first.Lines)
	assert.Empty(t, last.Lines)
}

func TestRunStopsOnACancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	engine := &ocrtest.Engine{}

	_, err := ocr.Run(ctx, subtitleWithImages(t), engine, "")

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, engine.Calls)
}

func TestRunRejectsInvalidText(t *testing.T) {
	t.Parallel()

	confidence := 1.5
	engine := &ocrtest.Engine{Text: ocr.Text{Lines: []string{"Read"}, Confidence: &confidence}}
	s := subtitleWithImages(t)

	report, err := ocr.Run(t.Context(), s, engine, "")

	var cueErr *ocr.CueError
	require.ErrorAs(t, err, &cueErr)
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	assert.Empty(t, report.Texts)
	first, _ := s.Cue(0)
	assert.Empty(t, first.Lines)
}

func TestTextValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		confidence *float64
		want       string
	}{
		{name: "none"},
		{name: "zero", confidence: new(0.0)},
		{name: "one", confidence: new(1.0)},
		{name: "above one", confidence: new(1.5), want: "stbox: invalid argument: The confidence must be from 0 to 1, got 1.5."},
		{name: "below zero", confidence: new(-0.1), want: "stbox: invalid argument: The confidence must be from 0 to 1, got -0.1."},
		{name: "not a number", confidence: new(math.NaN()), want: "stbox: invalid argument: The confidence must be from 0 to 1, got NaN."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ocr.Text{Confidence: tt.confidence}.Validate()

			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			var argErr *model.InvalidArgumentError
			require.ErrorAs(t, err, &argErr)
			require.EqualError(t, err, tt.want)
		})
	}
}
