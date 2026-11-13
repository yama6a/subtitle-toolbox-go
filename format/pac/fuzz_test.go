package pac_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "pac/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if ext := filepath.Ext(e.Name()); ext == ".pac" || ext == ".fpc" {
			f.Add(testfiles.Read(f, "pac/real/"+e.Name()))
		}
	}
	f.Add(file("Go<now>, he said<quietly"))
	f.Add(file("\x03Red\xFFtext\x1FC1 end\x1FCcomment"))

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := pac.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, pac.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
			var previous time.Duration
			for i, c := range s.All() {
				if err := c.Position.Validate(); err != nil {
					t.Fatalf("cue %d has an invalid position: %v", i, err)
				}
				if c.Start < previous || c.Start%time.Millisecond != 0 || c.End%time.Millisecond != 0 {
					t.Fatalf("cue %d has the times %v to %v", i, c.Start, c.End)
				}
				previous = c.Start
			}
			var out bytes.Buffer
			if _, err := pac.Write(&out, s, model.CommonWriteOptions{}, pac.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}
