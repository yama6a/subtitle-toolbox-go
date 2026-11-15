package webvtt_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "vtt/*.vtt", "vtt/real/*.vtt", "lenient/*.vtt")
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
			s, warnings, err := webvtt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, webvtt.ReadOptions{})
			if err != nil && !errors.Is(err, model.ErrParse) {
				t.Fatalf("Read(%q, lenient %t) returned %v", content, lenient, err)
			}
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			for i, c := range s.All() {
				if err := c.Position.Validate(); err != nil {
					t.Fatalf("Read(%q) gave cue %d the position %+v: %v", content, i, c.Position, err)
				}
			}
			if _, err := webvtt.Write(io.Discard, s, model.CommonWriteOptions{}, webvtt.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write of Read(%q) returned %v", content, err)
			}
		}
	})
}
