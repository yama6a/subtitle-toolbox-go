package csv_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, dir := range []string{"csv", realDir} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if ext := filepath.Ext(e.Name()); ext == ".csv" || ext == ".tsv" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		"start,end,text\n1,2,\"a\n",
		"a,1,2\nb,3,4\n",
		"Start Time,End Time,Subtitle\n1,2.5,a\n",
		"start\tend\ttext\n0\t2340\ta\n2340\t5100\tb\n",
		"start,duration,text\n99999:59:59,2,a\n",
		"start,end,text\n99999999999999999999:00:00:00,1,a\n00:00:1,0:5,b\n",
		"\"\"\"x\",1,2\r\r\n;;\t\x00\xff",
	} {
		f.Add([]byte(seed))
	}
	layouts := []csv.ReadOptions{
		{},
		{Columns: csv.Columns{Start: csv.ColumnAt(0), End: csv.ColumnAt(1), Text: csv.ColumnAt(2), NoHeader: true}, FrameRate: 25},
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, opts := range layouts {
			for _, lenient := range []bool{false, true} {
				s, warnings, err := csv.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, opts)
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
				if err := csv.Write(&out, s, model.CommonWriteOptions{}, csv.WriteOptions{}); err != nil {
					t.Fatalf("Write returned %v", err)
				}
			}
		}
	})
}
