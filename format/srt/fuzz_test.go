package srt_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "srt/*.srt", "srt/real/*.srt", "lenient/*.srt", "encoding/*.srt")
	for _, seed := range []string{
		"1\n00:00:01,000 --> 00:00:02,000",
		"1\n00:00:01,000 --> 00:00:0",
		"1\n00:00:01,000 ---> 00:00:02,000 X1:99999999999999999999 X2:1 Y1:2 Y2:3\n{\\an8\\i1}a\n{\\b1\n}b",
		"0:0:2\uff0c500 -> 0:0:1,5 foo align:start\nText\n\nmore\n2\n00:00:03,000 --> 00:00:04,000",
		"1\r\r\n0099999:59:59,9999 --> 07:03,920\r<today> <a b> &amp; caf\xe9\x00\x01",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, srt.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			if _, err := srt.Write(&out, s, model.CommonWriteOptions{}, srt.WriteOptions{}); err != nil {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}

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
