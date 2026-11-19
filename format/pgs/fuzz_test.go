package pgs_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "pgs/*.sup")
	valid := new(builder).end(0).Bytes()
	f.Add(append([]byte("XG"), valid[2:]...))
	f.Add(append(bytes.Clone(valid), "junk"...))
	f.Add([]byte("PG\x00\x00"))
	f.Add(new(builder).palette(0, 0, testPalette).Bytes()[:44])
	f.Add(new(builder).presentation(0, 720, 576, stateEpochStart, 0, obj{id: 7}).palette(0, 0, testPalette).
		segment(0, 0x15, []byte("\x00\x07\x00\xC0\x00\x00\x07\x00\x04\x00\x02\x01\x01\x00\x00")).end(0).Bytes())

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := pgs.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, pgs.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			for i, c := range s.All() {
				if c.Start < 0 {
					t.Fatalf("cue %d has a negative start", i)
				}
				if c.Image == nil || len(c.Lines) != 0 {
					t.Fatalf("cue %d is no image cue without lines", i)
				}
				if err := c.Image.Validate(); err != nil {
					t.Fatalf("cue %d has an invalid image: %v", i, err)
				}
				if c.End < c.Start {
					t.Fatalf("cue %d ends before it starts", i)
				}
			}
			if _, err := pgs.Write(io.Discard, s, model.CommonWriteOptions{}, pgs.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v, which is no ErrUnwritable", err)
			}
		}
	})
}
