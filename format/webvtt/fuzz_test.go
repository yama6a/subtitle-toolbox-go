package webvtt_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
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

func FuzzWebVTTStream(f *testing.F) {
	for _, rel := range fixtureFiles(f, ".vtt") {
		f.Add(testfiles.Read(f, rel))
	}
	for _, seed := range []string{
		"WEBVTT\n\n00:01.000 --> 00:02.000\nA\n\n00:03.000 --> 00:04.000\nB\n",
		"WEBVTT\r\r\n\r\r\n00:01.000 --> 00:02.000\r\r\nFirst\r\r\n",
		"\xef\xbb\xbfWEBVTT\n\nSTYLE\n::cue { color: red; }\n\nREGION\nid:a\n\nNOTE x\n\n00:01.000 --> 00:02.000 region:a\nA\n",
		"WEBVTT\n00:01.000 --> 00:02.000\nA\n",
		"\n \n  WEBVTT  title \nX-Meta: 1 \n\n00:01.000 --> 00:02.000\nA\n \n\t\n",
		"first line\n00:01.000 --> 00:02.000\nA\n\x1a\n",
		"WEBVTT\n\n00:01.000 --> 00:02.000\nA\x00\x01\n\nB\n",
		"WEBVTT\n\n00:01.000 --> 00:02.000\nA\n\nB\n\n00:03.000 --> 00:04.000\nC\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			compareWebVTT(t, content, wraps[0].wrap, lenient)
		}
		compareWebVTT(t, content, wraps[1].wrap, true)
	})
}
