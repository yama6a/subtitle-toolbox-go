package cavena890_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/cavena890"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "cavena890/real/*.890")

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := cavena890.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, cavena890.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			if _, err := cavena890.Write(&out, s, model.CommonWriteOptions{}, cavena890.WriteOptions{}); err != nil {
				t.Fatalf("Write of a subtitle that Read returned: %v", err)
			}
			if _, _, err := cavena890.Read(&out, model.CommonReadOptions{}, cavena890.ReadOptions{}); err != nil {
				t.Fatalf("Read of the output of Write: %v", err)
			}
		}
	})
}
