package sami_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, dir := range []string{realDir, "lenient"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".smi" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		"<SAMI><BODY><SYNC Start=0><P Class=ENCC>one<P Class=FRCC>un\n<SYNC Start=2000>&nbsp;\n</BODY></SAMI>",
		"<SYNC Start=-200><P>a&nbsp<SYNC Start=99999999999999999999><P>b<SYNC End=1><P>c",
		"<SAMI><HEAD><STYLE>.A{lang:x}</STYLE></HEAD><BODY><SYNC Start=1><P Class=B>b</BODY>",
		"<SAMI>\n<HEAD><TITLE>Walk</TITLE></HEAD>\n<BODY>\n<P>The ferry leaves.</P>\n</BODY>\n</SAMI>\n",
		"<SYNC Start=0><P Class=ENCC><b>open <i>tags</p></div></font><font color=red>x\xff\x00",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := sami.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, sami.ReadOptions{})
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
			if err := sami.Write(&out, s, model.CommonWriteOptions{}, sami.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}
