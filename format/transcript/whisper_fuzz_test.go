package transcript_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzReadWhisper(f *testing.F) {
	for _, dir := range []string{"whisper/real", "whisper"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".json" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	f.Add(testfiles.Read(f, "lenient/missing_segment_end.whisper.json"))
	f.Add(testfiles.Read(f, "speakers/whisper_cpp_diarize.json"))
	f.Add([]byte(twoSegmentsWithWords))
	for _, seed := range []string{
		`{"segments": [{"start": 0, "end": 1, "text": "a b", "words": [{"word": "b", "start": 0.5}, {"word": "a", "start": 0}]}]}`,
		`{"segments": [{"start": 2, "end": 1, "text": "a", "speaker": "x"}], "words": [{"word": "a", "start": "0.5", "end": null}, 5]}`,
		`{"segments": [{"start": 0, "end": 1, "text": "a", "words": [{"word": "a", "start": 1e12}]}]}`,
		`{"transcription": [{"offsets": {"from": 0, "to": 9}, "text": " a b", "tokens": [{"text": " a"}, {"text": "[_X_]"}, {"text": " b", "offsets": {"from": 3}}]}]}`,
		`{"word_segments": [], "segments": [{"start": 0, "end": 1, "text": "a", "words": null}]}`,
		`[1]`, `{`, ``,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			for _, opts := range []transcript.ReadOptions{{}, withWords} {
				common := model.CommonReadOptions{Lenient: lenient}
				s, warnings, err := transcript.ReadWhisper(bytes.NewReader(content), common, opts)
				if err != nil {
					var perr *model.ParseError
					if !errors.As(err, &perr) {
						t.Fatalf("ReadWhisper returned %v, which is no *ParseError", err)
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
				if err := transcript.WriteWhisper(&out, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
					if !errors.Is(err, model.ErrUnwritable) {
						t.Fatalf("WriteWhisper returned %v, which does not match ErrUnwritable", err)
					}
					continue
				}
				if _, _, err := transcript.ReadWhisper(bytes.NewReader(out.Bytes()), common, opts); err != nil {
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
