package mcc_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func FuzzRead(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "mcc/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".mcc" {
			f.Add(testfiles.Read(f, "mcc/real/"+e.Name()))
		}
	}
	for _, seed := range []string{
		"File Format=MacCaption_MCC V2.0\n",
		header2 + "00:00:01:00\tT0BS0B4F43ZZ72F4FC\n",
		header2 + "00:00:01:00\tT4\n00:00:01:00\tT49SV\n",
		header2 + "999998:00:00:00\t" + packet(0x942C) + "\n",
		"File Format=MacCaption_MCC V3.0\n",
		"File Format=MacCaption_MCC V1.0\r\nTime Code Rate=25DF\r\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := mcc.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, mcc.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
			if _, err := mcc.Write(io.Discard, s, model.CommonWriteOptions{}, mcc.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v, which does not match ErrUnwritable", err)
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
