package ebustl_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "stl/real/*.stl", "lenient/bad_time_code.stl")
	f.Add(append(gsiBlock(nil), ttiBlock(tti{text: "\x80Caf\xC2e\x81\x8A"})...))

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := ebustl.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, ebustl.ReadOptions{SubtractStartOfProgramme: lenient})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			for i, c := range s.All() {
				if c.Start < 0 {
					t.Fatalf("cue %d has a negative start", i)
				}
				if err := c.Position.Validate(); err != nil {
					t.Fatalf("cue %d has an invalid position: %v", i, err)
				}
			}
			if _, err := ebustl.Write(io.Discard, s, model.CommonWriteOptions{}, ebustl.WriteOptions{}); err != nil {
				var uerr *model.UnwritableError
				if !errors.As(err, &uerr) {
					t.Fatalf("Write returned %v, which is no *UnwritableError", err)
				}
			}
		}
	})
}
