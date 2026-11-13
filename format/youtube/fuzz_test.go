package youtube_test

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "youtube/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			f.Add(testfiles.Read(f, "youtube/real/"+e.Name()))
		}
	}
	f.Add(testfiles.Read(f, "lenient/credit_before_xml.srv3"))
	for _, seed := range []string{
		`<transcript><text start="1" dur="2">a &amp;#39; b</text><text start="x">c</text><text start="1"/></transcript>`,
		`<timedtext><text t="1" d="2e3">a</text><text t="1"/></timedtext>`,
		`<timedtext format="3"><head><pen id="1" b="1" fc="#ff0000"/><wp id="1" ap="2"/></head><body><p t="0" d="1000" wp="1"><s p="1">a</s><s t="500"> b</s></p></body></timedtext>`,
		`<timedtext format="3"><body><w id="1"/><p t="0" w="1" a="1">` + "\n" + `</p><p t="5" w="1"><s t="1e99">a</s></p></body></timedtext>`,
		`{"events": [{"tStartMs": 0, "id": 1}, {"tStartMs": 0, "dDurationMs": 1000, "wWinId": 1, "segs": [{"utf8": "a"}, {"utf8": " b", "tOffsetMs": 300}]}]}`,
		`{"events": [{"tStartMs": 1, "aAppend": 1, "segs": [{"utf8": "\n"}]}, {"tStartMs": "1"}, 5, {"tStartMs": 1e300}]}`,
		`{"events": {}}`, `{"events": []}`, `<tt/>`, `<`, `{`, ``,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			for _, opts := range []youtube.ReadOptions{{}, withWords} {
				common := model.CommonReadOptions{Lenient: lenient}
				s, warnings, err := youtube.Read(bytes.NewReader(content), common, opts)
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
				if err := youtube.Write(&out, s, model.CommonWriteOptions{}, youtube.WriteOptions{}); err != nil {
					if !errors.Is(err, model.ErrUnwritable) {
						t.Fatalf("Write returned %v, which does not match ErrUnwritable", err)
					}
					continue
				}
				if _, _, err := youtube.Read(bytes.NewReader(out.Bytes()), common, opts); err != nil {
					t.Fatalf("the output %s does not read: %v", out.Bytes(), err)
				}
			}
		}
	})
}

func checkCues(t *testing.T, s *model.Subtitle) {
	t.Helper()

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
		if err := c.ValidateWords(); err != nil {
			t.Fatalf("cue %d has invalid words: %v", i, err)
		}
	}
}
