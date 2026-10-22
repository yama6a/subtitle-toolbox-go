package ass_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, dir := range []string{"ass", realDir, "lenient", "editing", "quotes-in-tags", "ruby", "encoding"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if ext := filepath.Ext(e.Name()); ext == ".ass" || ext == ".ssa" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		eventsHeader + `Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\b1}a{\i1}b{\b0}c{\t(\1c&HFF&)}{\p1}m{\p0}\N\h`,
		eventsHeader + `Dialogue: 0,99999:59:59.00,99999:59:59.50,Default,,0,0,0,,{\k50}a{\k50}b`,
		eventsHeader + "Dialogue: 0,0:00:07,50,0:00:08,00,Default,,0,0,0,,a\nComment: 0,0:00:01.00,0:00:01.00,Default,,0,0,0,,n",
		"[Events]\nFormat: Start, End, 1, Text\nDialogue: 0:00:01.00,0:00:02.00,x,a\nhello",
		"[V4 Styles]\nStyle: Default,Arial,1,0,0,0,0,-1,1,1,1,0,10\n[Events]\nDialogue: Marked=0,0:00:01.00,0:00:00.5,Default,,0,0,0,,{\\a10\\rDefault}\xe9\x00\x01",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := ass.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, ass.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
			checkCues(t, s)
			for _, w := range warnings {
				if w.Action != model.Skipped && w.Action != model.Repaired {
					t.Fatalf("the warning %q has no action", w.Message)
				}
			}
			var out bytes.Buffer
			if err := ass.Write(&out, s, model.CommonWriteOptions{}, ass.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}

func checkCues(t *testing.T, s *model.Subtitle) {
	t.Helper()

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
}
