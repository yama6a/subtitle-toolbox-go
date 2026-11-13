package chapters_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func FuzzReadYouTube(f *testing.F) {
	for _, name := range []string{"video_description.txt", "long_stream_chapters.txt"} {
		f.Add(testfiles.Read(f, "chapters/youtube/real/"+name))
	}
	for _, seed := range []string{"0:00 Intro", "(2:48)x", "12:34:5x", "a—6:50\r\n1:2:03 x", "99999999999999999999:00:00", "[4:20] | x\x00"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := chapters.ReadYouTube(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, chapters.ReadOptions{})
			if err != nil {
				continue
			}
			for i, c := range s.All() {
				if c.End < c.Start {
					t.Fatalf("chapter %d ends before it starts", i)
				}
				if prev, ok := s.Cue(i - 1); ok && c.Start < prev.Start {
					t.Fatalf("chapter %d starts before chapter %d", i, i-1)
				}
				if c.Start < 0 || c.End >= timecode.MaxTime {
					t.Fatalf("chapter %d has the times %v %v", i, c.Start, c.End)
				}
			}
			var out bytes.Buffer
			if err := chapters.WriteYouTube(&out, s, model.CommonWriteOptions{}, chapters.WriteOptions{}); err != nil {
				t.Fatalf("WriteYouTube returned %v", err)
			}
		}
	})
}
