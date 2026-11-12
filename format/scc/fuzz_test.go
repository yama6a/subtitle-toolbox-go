package scc_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func FuzzRead(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "scc/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".scc" {
			f.Add(testfiles.Read(f, "scc/real/"+e.Name()))
		}
	}
	for _, seed := range []string{
		popOnHello,
		"Scenarist_SCC V1.0\n",
		"Scenarist_SCC V1.0\n\n00:00:01:00\t942c 94c\n",
		"Scenarist_SCC V1.0\r\n\r\n00:00:01:00\t942c\n\n00:00:02:00\t94zz\n",
		"Scenarist_SCC V1.0\n\n99:99:99;99\t9420 9420 9470 9470 c8e9 942f 942f\n00:00:00;00\t942c",
		"Scenarist_SCC V1.0\n\n942c 942c\n",
		"Scenarist_SCC V2.0\n",
		"Scenarist_SCC V1.0\n\n100000:05:59;29\t9420 9420 94ae 94ae 9452 9452 c8e9 942f 942f\n",
		"Scenarist_SCC V1.0\n\n100001:00:00:00\t9420 9470 c8e9 942f\n\n99999:59:59;29\t942c\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := scc.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, scc.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
			for _, fit := range []bool{false, true} {
				if _, err := scc.Write(io.Discard, s, model.CommonWriteOptions{}, scc.WriteOptions{Fit: fit}); err != nil && !errors.Is(err, model.ErrUnwritable) {
					t.Fatalf("Write returned %v, which does not match ErrUnwritable", err)
				}
			}
			var previous time.Duration
			for i, c := range s.Cues() {
				if i > 0 && c.Start < previous {
					t.Fatalf("cue %d starts before cue %d", i, i-1)
				}
				previous = c.Start
				for _, d := range []time.Duration{c.Start, c.End} {
					if d%time.Millisecond != 0 || d < 0 || d >= timecode.MaxTime {
						t.Fatalf("cue %d has the time %v", i, d)
					}
				}
			}
			for _, w := range warnings {
				if w.Action != model.Skipped && w.Action != model.Repaired {
					t.Fatalf("the warning %q has no action", w.Message)
				}
			}
		}
	})
}
