package mpsub_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "mpsub/*.mpsub", "mpsub/real/*.sub", "lenient/*.mpsub")
	for _, seed := range []string{
		"FORMAT=TIME\n\n1 2.5\nHello\n",
		"FORMAT=29.97\n\n29 58\nHello\n\nFORMAT=0\n",
		"FORMAT=25\n\n12.5 12.5\nA\r\n\r\n-3 1\nB",
		"TITLE=x # y\nNOTE=\n\n359999999.9996 0\nx\n",
		"\n-99999999999999999999999 1\nx\n# c\n1 -1\n1 2\n",
		"FORMAT=99999999999999999999\n\n1 1\n\xff\x00\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := mpsub.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, mpsub.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			for i, c := range s.All() {
				if c.End < c.Start || len(c.Lines) == 0 {
					t.Fatalf("cue %d is %v", i, c)
				}
			}
			checkWrite(t, s, mpsub.WriteOptions{})
			checkWrite(t, s, mpsub.WriteOptions{FrameRate: model.FrameRate{Num: 25, Den: 1}})
		}
	})
}

// checkWrite writes s and reads the output back. The times stay, except that a time below 0 becomes 0 and that
// frames round to a frame.
func checkWrite(t *testing.T, s *model.Subtitle, opts mpsub.WriteOptions) {
	t.Helper()

	var out bytes.Buffer
	if _, err := mpsub.Write(&out, s, model.CommonWriteOptions{}, opts); err != nil {
		t.Fatalf("Write returned %v", err)
	}
	back, _, err := mpsub.Read(&out, model.CommonReadOptions{}, mpsub.ReadOptions{})
	if err != nil {
		t.Fatalf("Read of the written output returned %v", err)
	}
	if back.Len() != s.Len() {
		t.Fatalf("the written output reads as %d cues, want %d", back.Len(), s.Len())
	}
	if !opts.FrameRate.IsZero() {
		return
	}
	for i, c := range s.All() {
		got := back.Cues()[i]
		if got.Start != max(c.Start, 0) || got.End != max(c.End, 0) {
			t.Fatalf("cue %d is %v to %v, want %v to %v", i, got.Start, got.End, max(c.Start, 0), max(c.End, 0))
		}
	}
}
