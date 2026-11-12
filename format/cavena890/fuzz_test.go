package cavena890_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/cavena890"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func FuzzRead(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "cavena890/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".890" {
			f.Add(testfiles.Read(f, "cavena890/real/"+e.Name()))
		}
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := cavena890.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, cavena890.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
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
