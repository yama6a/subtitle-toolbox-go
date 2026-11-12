package mp4_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/container/mp4"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

type cueRow struct {
	Start, End time.Duration
	Lines      []string
	Forced     bool
}

func rows(s *model.Subtitle) []cueRow {
	out := make([]cueRow, 0, len(s.Cues()))
	for _, c := range s.Cues() {
		out = append(out, cueRow{c.Start, c.End, c.Lines, c.Forced})
	}
	return out
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func openBytes(tb testing.TB, content []byte) *mp4.File {
	tb.Helper()
	f, err := mp4.Open(bytes.NewReader(content), int64(len(content)))
	require.NoError(tb, err)
	return f
}

func openFixture(tb testing.TB, name string) *mp4.File {
	tb.Helper()
	return openBytes(tb, testfiles.Read(tb, "mp4/"+name))
}

// single returns a file with one track that holds one sample of 1000 ms. The fields of spec replace the defaults.
func single(spec trackSpec, sample []byte) []byte {
	if spec.id == 0 {
		spec.id = 1
	}
	if spec.handler == "" {
		spec.handler = "sbtl"
	}
	if spec.entry == nil {
		spec.entry = tx3gEntry()
	}
	if spec.perChunk == 0 {
		spec.perChunk = 1
	}
	if spec.samples == nil {
		spec.samples = []sampleSpec{{1000, sample}}
	}
	return file([]trackSpec{spec}, true)
}

func extract(tb testing.TB, f *mp4.File, number int, opts mp4.ExtractOptions) *model.Subtitle {
	tb.Helper()
	s, warnings, err := f.Extract(number, model.CommonReadOptions{}, opts)
	require.NoError(tb, err)
	require.Nil(tb, warnings)
	return s
}
