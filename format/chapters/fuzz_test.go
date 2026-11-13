package chapters_test

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
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

// checkChapters fails when the chapters of s are not in start order or a time is outside 0 to 100000 hours.
// A chapter end can be before its start, because a file can give such an end.
func checkChapters(t *testing.T, s *model.Subtitle) {
	t.Helper()

	for i, c := range s.All() {
		if prev, ok := s.Cue(i - 1); ok && c.Start < prev.Start {
			t.Fatalf("chapter %d starts before chapter %d", i, i-1)
		}
		if c.Start <= -timecode.MaxTime || c.Start >= timecode.MaxTime || c.End <= -timecode.MaxTime || c.End >= timecode.MaxTime {
			t.Fatalf("chapter %d has the times %v %v", i, c.Start, c.End)
		}
	}
}

func fuzzRead(f *testing.F, rd reader, wr writer, folder string, seeds ...string) {
	f.Helper()

	for _, name := range testfilesIn(f, folder) {
		f.Add(testfiles.Read(f, name))
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := rd(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, chapters.ReadOptions{})
			if err != nil {
				continue
			}
			checkChapters(t, s)
			if err := wr(&bytes.Buffer{}, s, model.CommonWriteOptions{}, chapters.WriteOptions{}); err != nil {
				var uerr *model.UnwritableError
				if !errors.As(err, &uerr) {
					t.Fatalf("write returned %v", err)
				}
			}
		}
	})
}

func testfilesIn(f *testing.F, folder string) []string {
	f.Helper()

	entries, err := os.ReadDir(testfiles.Path(f, "chapters/"+folder+"/real"))
	if err != nil {
		f.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			names = append(names, "chapters/"+folder+"/real/"+e.Name())
		}
	}
	return names
}

func FuzzReadPodcast(f *testing.F) {
	fuzzRead(f, chapters.ReadPodcast, chapters.WritePodcast, "podcast",
		`{"chapters": [{"startTime": 1e400}]}`, `{"chapters": [{"startTime": 5, "endTime": 3, "title": "x", "a": {"b": [1.0]}}]}`, `[]`)
}

func FuzzReadFFMetadata(f *testing.F) {
	fuzzRead(f, chapters.ReadFFMetadata, chapters.WriteFFMetadata, "ffmetadata",
		";FFMETADATA1\n[CHAPTER]\nTIMEBASE=99999999999999999999/1\n", ";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/3\nSTART=-9223372036854775808\nEND=9223372036854775807\n",
		";FFMETADATA1\na\\\nb\\=c=d\\\n", ";FFMETADATA1\r[STREAM]\rx=y\r")
}

func FuzzReadOGM(f *testing.F) {
	fuzzRead(f, chapters.ReadOGM, chapters.WriteOGM, "ogm",
		"CHAPTER01=00:00:00.0005\nCHAPTER01NAME=x\nCHAPTER02=1", "CHAPTER1 = 99999999999999999999 : 0 : 0 , 1\nCHAPTER1NAME", "CHAPTER01=0:0:0.0\x00")
}
