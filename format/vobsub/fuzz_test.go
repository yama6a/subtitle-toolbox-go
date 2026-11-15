package vobsub_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "vobsub/two-tracks-pal.sub", "vobsub/custom-colors-ntsc.sub")
	f.Add(cutCommand)
	f.Add(pesOf(cutCommand))
	idx := testfiles.Read(f, "vobsub/two-tracks-pal.idx")

	f.Fuzz(func(t *testing.T, sub []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := vobsub.Read(bytes.NewReader(sub), model.CommonReadOptions{Lenient: lenient}, vobsub.ReadOptions{IDX: string(idx)})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			for i, c := range s.All() {
				if c.Image == nil || c.Image.Validate() != nil {
					t.Fatalf("cue %d has no valid image", i)
				}
			}
		}
	})
}
