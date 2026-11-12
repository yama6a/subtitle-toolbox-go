package pgs_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func FuzzRead(f *testing.F) {
	files, err := filepath.Glob(testfiles.Path(f, "pgs/*.sup"))
	if err != nil {
		f.Fatal(err)
	}
	for _, name := range files {
		f.Add(testfiles.Read(f, "pgs/"+filepath.Base(name)))
	}
	valid := new(builder).end(0).Bytes()
	f.Add(append([]byte("XG"), valid[2:]...))
	f.Add(append(bytes.Clone(valid), "junk"...))
	f.Add([]byte("PG\x00\x00"))
	f.Add(new(builder).palette(0, 0, testPalette).Bytes()[:44])
	f.Add(new(builder).presentation(0, 720, 576, stateEpochStart, 0, obj{id: 7}).palette(0, 0, testPalette).
		segment(0, 0x15, []byte("\x00\x07\x00\xC0\x00\x00\x07\x00\x04\x00\x02\x01\x01\x00\x00")).end(0).Bytes())

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := pgs.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, pgs.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
			var previous time.Duration
			for i, c := range s.All() {
				if c.Image == nil || len(c.Lines) != 0 {
					t.Fatalf("cue %d is no image cue without lines", i)
				}
				if err := c.Image.Validate(); err != nil {
					t.Fatalf("cue %d has an invalid image: %v", i, err)
				}
				if c.Start < previous || c.End < c.Start || c.Start%time.Millisecond != 0 || c.End%time.Millisecond != 0 || c.End >= timecode.MaxTime {
					t.Fatalf("cue %d has the times %v to %v", i, c.Start, c.End)
				}
				previous = c.Start
			}
		}
	})
}
