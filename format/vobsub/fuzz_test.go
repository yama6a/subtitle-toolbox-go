package vobsub_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func FuzzRead(f *testing.F) {
	f.Add(testfiles.Read(f, "vobsub/two-tracks-pal.sub"))
	f.Add(testfiles.Read(f, "vobsub/custom-colors-ntsc.sub"))
	f.Add(cutCommand)
	f.Add(pesOf(cutCommand))
	idx := testfiles.Read(f, "vobsub/two-tracks-pal.idx")

	f.Fuzz(func(t *testing.T, sub []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := vobsub.Read(bytes.NewReader(sub), model.CommonReadOptions{Lenient: lenient}, vobsub.ReadOptions{IDX: string(idx)})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
			previous := -timecode.MaxTime
			for i, c := range s.All() {
				if c.Image == nil || c.Image.Validate() != nil {
					t.Fatalf("cue %d has no valid image", i)
				}
				if c.Start < previous || c.Start%time.Millisecond != 0 || c.End%time.Millisecond != 0 || c.End >= timecode.MaxTime {
					t.Fatalf("cue %d has the times %v to %v", i, c.Start, c.End)
				}
				previous = c.Start
			}
		}
	})
}
