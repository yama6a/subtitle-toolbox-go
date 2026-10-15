package writing_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzFinish(f *testing.F) {
	f.Add([]byte("\ufeff1\n00:00:01,000 --> 00:00:02,000\r\nHi\r\n"), uint8(1), uint8(2), true)
	f.Add([]byte("a\rb\n\n"), uint8(0), uint8(1), false)

	f.Fuzz(func(t *testing.T, body []byte, lineEnding, bomRule uint8, formatBOM bool) {
		common := model.CommonWriteOptions{LineEnding: model.LineEnding(lineEnding % 2), BOM: model.BOM(bomRule % 3)}
		got := writing.Finish(body, common, formatBOM)
		if common.LineEnding == model.CRLF {
			for i, b := range got {
				if b == '\n' && (i == 0 || got[i-1] != '\r') {
					t.Fatalf("Finish(%q) left a LF without CR: %q", body, got)
				}
			}
		}
		hasBOM := bytes.HasPrefix(got, []byte(bom))
		switch {
		case common.BOM == model.BOMRemove && hasBOM:
			t.Fatalf("Finish(%q) kept a leading BOM", body)
		case common.BOM == model.BOMAdd && !hasBOM, common.BOM == model.BOMDefault && formatBOM && !hasBOM:
			t.Fatalf("Finish(%q) has no BOM", body)
		case bytes.HasPrefix(got, []byte(bom+bom)) && !bytes.HasPrefix(body, []byte(bom+bom)):
			t.Fatalf("Finish(%q) added a second BOM", body)
		}
	})
}
