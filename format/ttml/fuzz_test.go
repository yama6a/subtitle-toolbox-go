package ttml_test

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, dir := range []string{realDir, "forced", "lenient"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if !e.IsDir() && e.Name() != "SOURCES.md" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		document(`<div begin="10s" end="20s"><p begin="1s" end="2s" dur="5s">a<span begin="0.5s">b</span></p><p begin="30s">c</p></div>`, "", ""),
		document(`<div><p begin="00:00:01:45" end="99999:59:59.9999">a&nbsp;&#13;</p><p begin="5s" end="2s">x</p></div>`,
			`ttp:timeBase="smpte" ttp:frameRate="-0.001" ttp:dropMode="dropNTSC"`, ""),
		document(`<div region="r"><p end="1s" style="a" ttm:agent="x">x<br>y</p>`, "", `<styling><style xml:id="a" style="a"/></styling>`),
		"Credit\n<?xml version=\"1.0\"?><tt><body><div><p begin=\"1\" dur=\"1:2\">a & b</div></body></tt>",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := ttml.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, ttml.ReadOptions{})
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
			for _, w := range warnings {
				if w.Action != model.Skipped && w.Action != model.Repaired {
					t.Fatalf("the warning %q has no action", w.Message)
				}
			}
			var out bytes.Buffer
			if err := ttml.Write(&out, s, model.CommonWriteOptions{}, ttml.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}
