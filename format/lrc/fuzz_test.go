package lrc_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/lrc"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, dir := range []string{"lrc", realDir, "karaoke", "lenient"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".lrc" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		"[00:12.00][01:15.30]Chorus\n[00:17.20]Verse\n",
		"[00:01.00] <00:01.00> Fish <00:01.50> chips <00:02.00>\n",
		"[00:01.00]<00:01>Fish & <00:01.50>chips <3 <1:2>\n",
		"[offset:-99999999999999999999]\n[00:12.00]x\n",
		"[offset:-359999987999]\n[00:12.00]x <00:12.00>y\n",
		"[00:02:35]\nHello\n[#:note]\n[ti:x]\n[00:1x.00]y\n",
		"[ti:Harbour walk]\nThe ferry leaves at nine.\n",
		"[00:01.00]caf\xE9\x00\x0b & tea\r\r\n\t[ 1:2:03.5 ]",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := lrc.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, lrc.ReadOptions{})
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
			if err := lrc.Write(&out, s, model.CommonWriteOptions{}, lrc.WriteOptions{}); err != nil {
				t.Fatalf("Write returned %v", err)
			}
			if _, _, err := lrc.Read(&out, model.CommonReadOptions{}, lrc.ReadOptions{}); err != nil {
				t.Fatalf("Read of the written output returned %v", err)
			}
		}
	})
}
