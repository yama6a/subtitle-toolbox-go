package microdvd_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, dir := range []string{"microdvd", realDir, "lenient", "cli"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".sub" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		"{1}{1}0\n{25}{50}Hello",
		"{1}{1}Hello",
		"{24}{}Hello",
		"{24}{99999999999999999999}Hello",
		"One|{Y:u}Two",
		"{0}{25}{C:$0000FF}{Y:i}One|{c:$00FF00}{y:b}Two",
		"{0}{25}{y:i}caf\xe9 & bread|<3 jam",
		"{359999999999650}{359999999999650}x",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := microdvd.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, microdvd.ReadOptions{FrameRate: 25})
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
			var opts microdvd.WriteOptions
			if _, ok := s.FormatData("microdvd"); !ok {
				opts.FrameRate = 25 // a blank input gives no frame rate to write with
			}
			var out bytes.Buffer
			if err := microdvd.Write(&out, s, model.CommonWriteOptions{}, opts); err != nil && !errors.Is(err, model.ErrUnwritable) {
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
