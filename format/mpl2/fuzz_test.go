package mpl2_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/mpl2"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, dir := range []string{"mpl2", realDir, "mpl2/lenient"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".txt" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		"[12][45]Where are you?|/Home.",
		"[99999999999999999999][1]x",
		"[3599999999][3599999999]/|/ |//x",
		"[20][10]a\r\r\n[1][2]<i>b</i>\x00\xff",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := mpl2.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, mpl2.ReadOptions{})
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
			var out bytes.Buffer
			if err := mpl2.Write(&out, s, model.CommonWriteOptions{}, mpl2.WriteOptions{}); err != nil {
				t.Fatalf("Write returned %v", err)
			}
			back, _, err := mpl2.Read(&out, model.CommonReadOptions{}, mpl2.ReadOptions{})
			if err != nil {
				t.Fatalf("Read of the written output returned %v", err)
			}
			if back.Len() != s.Len() {
				t.Fatalf("the written output reads as %d cues, want %d", back.Len(), s.Len())
			}
		}
	})
}
