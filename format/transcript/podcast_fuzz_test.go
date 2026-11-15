package transcript_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzReadPodcast(f *testing.F) {
	formattest.AddSeeds(f, "podcast/real/*.json")
	f.Add([]byte(dialogueWords))
	for _, seed := range []string{
		`{"segments": [{"startTime": 5, "endTime": 6, "body": "a"}, {"startTime": 1, "endTime": 2, "body": "b"}]}`,
		`{"segments": [{"startTime": 2, "endTime": 1, "speaker": "x", "body": "a b"}, {"startTime": 1, "body": "c"}, "x", {"startTime": 1e12}]}`,
		`{"segments": [{"startTime": 0, "body": "<b>&amp;"}, {"startTime": 0, "body": "a\f"}], "version": null}`,
		`{"segments": [{"startTime": 0, "endTime": 1, "body": "a", "speaker": null, "x": [1]}]}`,
		`[1]`, `{`, ``,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			for _, opts := range []transcript.ReadOptions{{}, withWords, {KeepSegments: true, WordTimestamps: true}} {
				common := model.CommonReadOptions{Lenient: lenient}
				s, warnings, err := transcript.ReadPodcast(bytes.NewReader(content), common, opts)
				if !formattest.CheckRead(t, s, warnings, err) {
					continue
				}
				checkWords(t, s)
				for _, wordSegments := range []bool{false, true} {
					var out bytes.Buffer
					w := transcript.WriteOptions{Podcast: transcript.PodcastWriteOptions{WordSegments: wordSegments}}
					if err := transcript.WritePodcast(&out, s, model.CommonWriteOptions{}, w); err != nil {
						if !errors.Is(err, model.ErrUnwritable) {
							t.Fatalf("WritePodcast returned %v, which does not match ErrUnwritable", err)
						}
						continue
					}
					if _, _, err := transcript.ReadPodcast(bytes.NewReader(out.Bytes()), common, opts); err != nil {
						t.Fatalf("the output %s does not read: %v", out.Bytes(), err)
					}
				}
			}
		}
	})
}
