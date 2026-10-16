package stream_test

import (
	"bytes"
	"cmp"
	"errors"
	"io"
	"iter"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/stream"
)

// wraps are the readers that cut the input at other places than the file does.
var wraps = []struct {
	name string
	wrap func(io.Reader) io.Reader
}{
	{"whole", func(r io.Reader) io.Reader { return r }},
	{"one byte", iotest.OneByteReader},
	{"half", iotest.HalfReader},
	{"data with error", iotest.DataErrReader},
}

func collect(seq iter.Seq2[model.Cue, error]) ([]model.Cue, error) {
	var cues []model.Cue
	for c, err := range seq {
		if err != nil {
			return cues, err
		}
		cues = append(cues, c)
	}
	return cues, nil
}

// inStartOrder puts cues through a subtitle, which sorts them by start as the full readers do.
func inStartOrder(cues []model.Cue) []model.Cue {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return cuesOf(s)
}

func cuesOf(s *model.Subtitle) []model.Cue {
	var cues []model.Cue
	for _, c := range s.All() {
		cues = append(cues, c)
	}
	return cues
}

func sortedByLine(w []model.ParseWarning) []model.ParseWarning {
	w = slices.Clone(w)
	slices.SortStableFunc(w, func(a, b model.ParseWarning) int { return cmp.Compare(a.Line, b.Line) })
	return w
}

// compareSubRip checks that the stream reader gives the cues, warnings and first error of srt.Read.
func compareSubRip(tb testing.TB, content []byte, wrap func(io.Reader) io.Reader, lenient bool) {
	tb.Helper()

	s, warnings, fullErr := srt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, srt.ReadOptions{})
	rd := stream.NewSubRipReader(wrap(bytes.NewReader(content)), stream.ReadOptions{Lenient: lenient})
	cues, err := collect(rd.Cues())
	if fullErr != nil {
		compareErrors(tb, fullErr, err)
		return
	}
	require.NoError(tb, err)
	assert.Equal(tb, cuesOf(s), inStartOrder(cues))
	assert.Equal(tb, warnings, rd.Warnings())
}

// compareWebVTT checks that the stream reader gives the cues, header, warnings and first error of webvtt.Read.
func compareWebVTT(tb testing.TB, content []byte, wrap func(io.Reader) io.Reader, lenient bool) {
	tb.Helper()

	s, warnings, fullErr := webvtt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, webvtt.ReadOptions{})
	rd := stream.NewWebVTTReader(wrap(bytes.NewReader(content)), stream.ReadOptions{Lenient: lenient})
	cues, err := collect(rd.Cues())
	if fullErr != nil {
		compareErrors(tb, fullErr, err)
		return
	}
	require.NoError(tb, err)
	assert.Equal(tb, cuesOf(s), inStartOrder(cues))
	assert.Equal(tb, sortedByLine(warnings), rd.Warnings())
	header, ok := s.FormatData(string(model.WebVTT))
	if !ok {
		header = model.NewData()
	}
	assert.Equal(tb, header, rd.Header())
}

// compareErrors checks the error of the stream reader. Strict SubRip reads a NUL byte as an error before it reads
// any block, so a block error on a line before the NUL line may come first from the stream reader. Every other
// comparison needs the same error.
func compareErrors(tb testing.TB, want, got error) {
	tb.Helper()

	require.Error(tb, got)
	var wantErr, gotErr *model.ParseError
	if errors.As(want, &wantErr) && strings.Contains(wantErr.Msg, "has a NUL character") &&
		errors.As(got, &gotErr) && gotErr.Msg != wantErr.Msg {
		assert.Less(tb, gotErr.Line, wantErr.Line)
		return
	}
	assert.Equal(tb, want.Error(), got.Error())
}
