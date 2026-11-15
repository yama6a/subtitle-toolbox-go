package pac_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "pac/real/*.pac", "pac/real/*.fpc")
	f.Add(file("Go<now>, he said<quietly"))
	f.Add(file("\x03Red\xFFtext\x1FC1 end\x1FCcomment"))

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := pac.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, pac.ReadOptions{})
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
			var out bytes.Buffer
			if _, err := pac.Write(&out, s, model.CommonWriteOptions{}, pac.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}
