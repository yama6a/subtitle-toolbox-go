package transcript_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzReadWhisper(f *testing.F) {
	formattest.AddSeeds(f, "whisper/real/*.json", "whisper/*.json")
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
				if !formattest.CheckRead(t, s, warnings, err) {
					continue
				}
				checkWords(t, s)
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

func checkWords(t *testing.T, s *model.Subtitle) {
	t.Helper()

	for i, c := range s.Cues() {
		if err := c.ValidateWords(); err != nil {
			t.Fatalf("cue %d has invalid words: %v", i, err)
		}
	}
}
