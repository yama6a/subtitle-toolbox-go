package ebustl_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func FuzzRead(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "stl/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".stl" {
			f.Add(testfiles.Read(f, "stl/real/"+e.Name()))
		}
	}
	f.Add(testfiles.Read(f, "lenient/bad_time_code.stl"))
	f.Add(append(gsiBlock(nil), ttiBlock(tti{text: "\x80Caf\xC2e\x81\x8A"})...))

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := ebustl.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, ebustl.ReadOptions{SubtractStartOfProgramme: lenient})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
			var previous time.Duration
			for i, c := range s.All() {
				if err := c.Position.Validate(); err != nil {
					t.Fatalf("cue %d has an invalid position: %v", i, err)
				}
				if c.Start < previous || c.Start%time.Millisecond != 0 || c.End%time.Millisecond != 0 {
					t.Fatalf("cue %d has the times %v to %v", i, c.Start, c.End)
				}
				previous = c.Start
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
