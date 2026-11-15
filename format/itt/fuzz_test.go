package itt_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "itt/real/*")
	f.Add(testfiles.Read(f, "forced/forced_signs_2398.itt"))
	f.Add([]byte(`<tt xmlns="http://www.w3.org/ns/ttml" ttp:frameRate="1e300" ttp:frameRateMultiplier="3 0"><body><div><p begin="1s" end="2s">a</p></div></body></tt>`))

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := itt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, itt.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			_, err = itt.Write(&out, s, model.CommonWriteOptions{}, itt.WriteOptions{FrameRate: model.FrameRate25})
			if err != nil {
				if !errors.Is(err, model.ErrUnwritable) {
					t.Fatalf("Write returned %v", err)
				}
				continue
			}
			if _, _, err := itt.Read(&out, model.CommonReadOptions{}, itt.ReadOptions{}); err != nil {
				t.Fatalf("the output does not read back: %v", err)
			}
		}
	})
}
