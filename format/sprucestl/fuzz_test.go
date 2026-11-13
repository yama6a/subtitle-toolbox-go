package sprucestl_test

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/sprucestl"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "spruce-stl/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			f.Add(testfiles.Read(f, "spruce-stl/real/"+e.Name()))
		}
	}
	for _, seed := range []string{
		"00:00:01:15 , 00:00:02:25 , NTSC\n",
		"$Italic = TRUE\n00:00:01:00 , 00:00:02:00 , Sung ^Ispoken^I sung|still\n$Bold = True\n",
		"00:00:01:25 , 00:00:02:00 , Late\n",
		"99999999999999999999:00:00:00 , 00:00:00:00 , x\n",
		"00:00:02:00 , 00:00:01:00 , Back\n//note\n",
		"$HorzAlign = Left\n$VertAlign = Top\n00:00:01:00,00:00:02:00,^B^^I|^\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := sprucestl.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, sprucestl.ReadOptions{})
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
			var out bytes.Buffer
			if _, err := sprucestl.Write(&out, s, model.CommonWriteOptions{}, sprucestl.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}
