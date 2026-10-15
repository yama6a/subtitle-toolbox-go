package webvtt_test

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, pattern := range []string{"vtt/*.vtt", "vtt/real/*.vtt", "lenient/*.vtt"} {
		paths, err := filepath.Glob(testfiles.Path(f, pattern))
		if err != nil {
			f.Fatal(err)
		}
		for _, p := range paths {
			rel, _ := filepath.Rel(testfiles.Root(f), p)
			f.Add(testfiles.Read(f, rel))
		}
	}
	for _, s := range []string{
		"WEBVTT",
		"WEBVTTX\n\n00:01.000 --> 00:02.000\nHi\n",
		"WEBVTT\nKind: captions\n00:01.000 --> 00:02.000\nHi\n00:03.000 --> 00:04.000\nHo\n",
		"WEBVTT\n\n00:01.000 --> 00:01.000line:40% region:bill align:right\n<00:03.000>Rye & spelt &foo; <today>\n",
		"WEBVTT\n\n00:00:01.5 --> 00:00:0\n\nstray\n\nNOTE-1\n00:05.000 --> 00:01.000\nx\n\nSTYLE\na\n",
		"\n\nhello\n1\n00:00:01.000 --> 00:00:02.000\n<100000:00:00.000>\n",
		"WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nthe<00:00:01.500><c> ferry</c>\n\n00:00:02.000 --> 00:00:02.010\nthe ferry\n",
	} {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, _, err := webvtt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, webvtt.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.Is(err, model.ErrParse) || !errors.As(err, &perr) {
					t.Fatalf("Read(%q, lenient %t) returned %v", content, lenient, err)
				}
				continue
			}
			var last time.Duration
			for i, c := range s.All() {
				if i > 0 && c.Start < last {
					t.Fatalf("Read(%q) gave cue %d at %s before %s", content, i, c.Start, last)
				}
				last = c.Start
				for _, d := range []time.Duration{c.Start, c.End} {
					if d%time.Millisecond != 0 || d < 0 || d >= timecode.MaxTime {
						t.Fatalf("Read(%q) gave cue %d the time %s", content, i, d)
					}
				}
			}
			if err := webvtt.Write(io.Discard, s, model.CommonWriteOptions{}, webvtt.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write of Read(%q) returned %v", content, err)
			}
		}
	})
}
