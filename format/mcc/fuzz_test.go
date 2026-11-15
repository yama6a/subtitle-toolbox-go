package mcc_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "mcc/real/*.mcc")
	for _, seed := range []string{
		"File Format=MacCaption_MCC V2.0\n",
		header2 + "00:00:01:00\tT0BS0B4F43ZZ72F4FC\n",
		header2 + "00:00:01:00\tT4\n00:00:01:00\tT49SV\n",
		header2 + "999998:00:00:00\t" + packet(0x942C) + "\n",
		"File Format=MacCaption_MCC V3.0\n",
		"File Format=MacCaption_MCC V1.0\r\nTime Code Rate=25DF\r\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := mcc.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, mcc.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			for i, c := range s.Cues() {
				if c.Start < 0 || c.End < 0 {
					t.Fatalf("cue %d has a negative time", i)
				}
			}
			if _, err := mcc.Write(io.Discard, s, model.CommonWriteOptions{}, mcc.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v, which does not match ErrUnwritable", err)
			}
		}
	})
}
