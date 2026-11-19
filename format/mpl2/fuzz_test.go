package mpl2_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/mpl2"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "mpl2/*.txt", "mpl2/real/*.txt", "mpl2/lenient/*.txt")
	for _, seed := range []string{
		"[12][45]Where are you?|/Home.",
		"[99999999999999999999][1]x",
		"[3599999999][3599999999]/|/ |//x",
		"[20][10]a\r\r\n[1][2]<i>b</i>\x00\xff",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := mpl2.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, mpl2.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			if _, err := mpl2.Write(&out, s, model.CommonWriteOptions{}, mpl2.WriteOptions{}); err != nil {
				t.Fatalf("Write returned %v", err)
			}
			back, _, err := mpl2.Read(&out, model.CommonReadOptions{}, mpl2.ReadOptions{})
			if err != nil {
				t.Fatalf("Read of the written output returned %v", err)
			}
			if back.Len() != s.Len() {
				t.Fatalf("the written output reads as %d cues, want %d", back.Len(), s.Len())
			}
		}
	})
}
