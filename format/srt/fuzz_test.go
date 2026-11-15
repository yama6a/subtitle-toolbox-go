package srt_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "srt/*.srt", "srt/real/*.srt", "lenient/*.srt", "encoding/*.srt")
	for _, seed := range []string{
		"1\n00:00:01,000 --> 00:00:02,000",
		"1\n00:00:01,000 --> 00:00:0",
		"1\n00:00:01,000 ---> 00:00:02,000 X1:99999999999999999999 X2:1 Y1:2 Y2:3\n{\\an8\\i1}a\n{\\b1\n}b",
		"0:0:2\uff0c500 -> 0:0:1,5 foo align:start\nText\n\nmore\n2\n00:00:03,000 --> 00:00:04,000",
		"1\r\r\n0099999:59:59,9999 --> 07:03,920\r<today> <a b> &amp; caf\xe9\x00\x01",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, srt.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			if _, err := srt.Write(&out, s, model.CommonWriteOptions{}, srt.WriteOptions{}); err != nil {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}

func checkCues(t *testing.T, s *model.Subtitle) {
	t.Helper()

	var previous time.Duration
	for i, c := range s.All() {
		if i > 0 && c.Start < previous {
			t.Fatalf("cue %d starts before cue %d", i, i-1)
		}
		previous = c.Start
		for _, d := range []time.Duration{c.Start, c.End} {
			if d%time.Millisecond != 0 || d < 0 || d >= timecode.MaxTime {
				t.Fatalf("cue %d has the time %v", i, d)
			}
		}
	}
}
