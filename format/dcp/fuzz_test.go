package dcp_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/dcp"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "dcp/real/*")

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := dcp.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, dcp.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			for i, c := range s.All() {
				if err := c.Position.Validate(); err != nil {
					t.Fatalf("cue %d has the position %+v: %v", i, c.Position, err)
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
