package stream_test

import (
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

func FuzzSubRipStream(f *testing.F) {
	for _, rel := range fixtureFiles(f, ".srt") {
		f.Add(testfiles.Read(f, rel))
	}
	for _, seed := range []string{
		"1\n00:00:01,000 --> 00:00:02,000\nA\n\n2\n00:00:03,000 --> 00:00:04,000\nB",
		"1\r\r\n00:00:01,000 --> 00:00:02,000\r\rA\r\r\n",
		"1\r00:00:01,000 --> 00:00:02,000\rA\r\r2\r00:00:03,000 --> 00:00:04,000\rB\r",
		"\xef\xbb\xbf1\n00:00:01,000 --> 00:00:02,000\nA\n\x1a\n",
		"1\n00:00:01,000 --> 00:00:02,000\nA\x00\x01\x7f\n",
		"1\n00:00:01,000 --> 00:00:02,000\nbad \xff byte\n",
		"1\n00:00:01,000 --> 00:00:02,000\nA\n\n\n3\n00:00:03,000 --> 00:00:04,000\nB\n \x1a\n\t\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			compareSubRip(t, content, wraps[0].wrap, lenient)
		}
		compareSubRip(t, content, wraps[1].wrap, true)
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
