package dcp_test

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/dcp"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "dcp/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			f.Add(testfiles.Read(f, "dcp/real/"+e.Name()))
		}
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := dcp.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, dcp.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
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
				if err := c.Position.Validate(); err != nil {
					t.Fatalf("cue %d has a bad position: %v", i, err)
				}
			}
			var out bytes.Buffer
			if _, err := dcp.Write(&out, s, model.CommonWriteOptions{}, dcp.WriteOptions{}); err != nil {
				if !errors.Is(err, model.ErrUnwritable) {
					t.Fatalf("Write returned %v", err)
				}
				continue
			}
			if _, _, err := dcp.Read(&out, model.CommonReadOptions{}, dcp.ReadOptions{}); err != nil {
				t.Fatalf("the output does not read back: %v", err)
			}
		}
	})
}
