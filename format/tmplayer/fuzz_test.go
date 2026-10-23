package tmplayer_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/tmplayer"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, dir := range []string{realDir, "tmplayer/lenient", "lenient"} {
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
		"00:00:01:Where are you?|Home.",
		"99999999999999999999:00:04:x",
		"99999:59:59:x\n00:00:01,2=y\n00:00:01,99999999999999999999=z",
		"0:00:01=a\r\r\n00:00:01,1:b\x00\xff<i>",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := tmplayer.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, tmplayer.ReadOptions{})
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
			if err := tmplayer.Write(&out, s, model.CommonWriteOptions{}, tmplayer.WriteOptions{}); err != nil {
				t.Fatalf("Write returned %v", err)
			}
			if _, _, err := tmplayer.Read(&out, model.CommonReadOptions{}, tmplayer.ReadOptions{}); err != nil {
				t.Fatalf("Read of the written output returned %v", err)
			}
		}
	})
}
