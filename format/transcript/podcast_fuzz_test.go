package transcript_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzReadPodcast(f *testing.F) {
	entries, err := os.ReadDir(testfiles.Path(f, "podcast/real"))
	if err != nil {
		f.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			f.Add(testfiles.Read(f, "podcast/real/"+e.Name()))
		}
	}
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
				s, _, err := transcript.ReadPodcast(bytes.NewReader(content), common, opts)
				if err != nil {
					var perr *model.ParseError
					if !errors.As(err, &perr) {
						t.Fatalf("ReadPodcast returned %v, which is no *ParseError", err)
					}
					continue
				}
				checkCues(t, s)
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
