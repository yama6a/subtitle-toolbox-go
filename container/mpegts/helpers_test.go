package mpegts_test

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/container/mpegts"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

type cueRow struct {
	Start, End time.Duration
	Lines      []string
}

// sec returns seconds as a duration of whole milliseconds.
func sec(s float64) time.Duration {
	return time.Duration(math.Round(s*1000)) * time.Millisecond
}

func cue(start, end float64, lines ...string) cueRow {
	return cueRow{sec(start), sec(end), lines}
}

func cuesOf(s *model.Subtitle) []cueRow {
	rows := make([]cueRow, 0, len(s.Cues()))
	for _, c := range s.Cues() {
		rows = append(rows, cueRow{c.Start, c.End, c.Lines})
	}
	return rows
}

func open(tb testing.TB, ts []byte) *mpegts.File {
	tb.Helper()

	f, err := mpegts.Open(bytes.NewReader(ts), int64(len(ts)))
	require.NoError(tb, err)
	return f
}

func extract(tb testing.TB, ts []byte, number int, opts model.CommonReadOptions) []cueRow {
	tb.Helper()

	s, warnings, err := open(tb, ts).Extract(number, opts)
	require.NoError(tb, err)
	require.Empty(tb, warnings)
	return cuesOf(s)
}

func fixture(tb testing.TB, name string) []byte {
	tb.Helper()

	return testfiles.Read(tb, "mpegts/"+name)
}

// split returns the 188-byte packets of a transport stream.
func split(ts []byte) [][]byte {
	var packets [][]byte
	for i := 0; i+188 <= len(ts); i += 188 {
		packets = append(packets, bytes.Clone(ts[i:i+188]))
	}
	return packets
}

func pid(p []byte) int { return int(p[1]&0x1F)<<8 | int(p[2]) }

func join(packets [][]byte) []byte { return bytes.Join(packets, nil) }

// single returns a writer with the tables of a program that has a teletext stream with the pages.
func single(pages ...ttPage) *tsWriter {
	w := newWriter()
	w.tables(teletextPID, teletextStream(teletextPID, pages...))
	return w
}

func eng888() ttPage { return ttPage{"eng", subtitlePageType, 888} }
