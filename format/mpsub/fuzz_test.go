package mpsub_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, dir := range []string{"mpsub", realDir, "lenient"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if ext := filepath.Ext(e.Name()); ext == ".mpsub" || ext == ".sub" && dir != "lenient" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		"FORMAT=TIME\n\n1 2.5\nHello\n",
		"FORMAT=29.97\n\n29 58\nHello\n\nFORMAT=0\n",
		"FORMAT=25\n\n12.5 12.5\nA\r\n\r\n-3 1\nB",
		"TITLE=x # y\nNOTE=\n\n359999999.9996 0\nx\n",
		"\n-99999999999999999999999 1\nx\n# c\n1 -1\n1 2\n",
		"FORMAT=99999999999999999999\n\n1 1\n\xff\x00\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := mpsub.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, mpsub.ReadOptions{})
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
					if d%time.Millisecond != 0 || d <= -timecode.MaxTime || d >= timecode.MaxTime {
						t.Fatalf("cue %d has the time %v", i, d)
					}
				}
				if c.End < c.Start || len(c.Lines) == 0 {
					t.Fatalf("cue %d is %v", i, c)
				}
			}
			for _, w := range warnings {
				if w.Action != model.Skipped && w.Action != model.Repaired {
					t.Fatalf("the warning %q has no action", w.Message)
				}
			}
			checkWrite(t, s, mpsub.WriteOptions{})
			checkWrite(t, s, mpsub.WriteOptions{FrameRate: 25})
		}
	})
}

// checkWrite writes s and reads the output back. The times stay, except that a time below 0 becomes 0 and that
// frames round to a frame.
func checkWrite(t *testing.T, s *model.Subtitle, opts mpsub.WriteOptions) {
	t.Helper()

	var out bytes.Buffer
	if err := mpsub.Write(&out, s, model.CommonWriteOptions{}, opts); err != nil {
		t.Fatalf("Write returned %v", err)
	}
	back, _, err := mpsub.Read(&out, model.CommonReadOptions{}, mpsub.ReadOptions{})
	if err != nil {
		t.Fatalf("Read of the written output returned %v", err)
	}
	if back.Len() != s.Len() {
		t.Fatalf("the written output reads as %d cues, want %d", back.Len(), s.Len())
	}
	if opts.FrameRate != 0 {
		return
	}
	for i, c := range s.All() {
		got := back.Cues()[i]
		if got.Start != max(c.Start, 0) || got.End != max(c.End, 0) {
			t.Fatalf("cue %d is %v to %v, want %v to %v", i, got.Start, got.End, max(c.Start, 0), max(c.End, 0))
		}
	}
}
