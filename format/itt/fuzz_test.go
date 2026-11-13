package itt_test

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "itt/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			f.Add(testfiles.Read(f, "itt/real/"+e.Name()))
		}
	}
	f.Add(testfiles.Read(f, "forced/forced_signs_2398.itt"))
	f.Add([]byte(`<tt xmlns="http://www.w3.org/ns/ttml" ttp:frameRate="1e300" ttp:frameRateMultiplier="3 0"><body><div><p begin="1s" end="2s">a</p></div></body></tt>`))

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := itt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, itt.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
			var previous time.Duration
			for i, c := range s.All() {
				if i > 0 && c.Start < previous {
					t.Fatalf("cue %d starts before cue %d", i, i-1)
				}
				previous = c.Start
				for _, d := range []time.Duration{c.Start, c.End} {
					if d%time.Millisecond != 0 || d >= timecode.MaxTime || d <= -timecode.MaxTime {
						t.Fatalf("cue %d has the time %v", i, d)
					}
				}
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
