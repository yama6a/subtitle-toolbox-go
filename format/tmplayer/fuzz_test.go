package tmplayer_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/tmplayer"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "tmplayer/real/*.txt", "tmplayer/lenient/*.txt", "lenient/*.txt")
	for _, seed := range []string{
		"00:00:01:Where are you?|Home.",
		"99999999999999999999:00:04:x",
		"99999:59:59:x\n00:00:01,2=y\n00:00:01,99999999999999999999=z",
		"0:00:01=a\r\r\n00:00:01,1:b\x00\xff<i>",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := tmplayer.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, tmplayer.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			if _, err := tmplayer.Write(&out, s, model.CommonWriteOptions{}, tmplayer.WriteOptions{}); err != nil {
				t.Fatalf("Write returned %v", err)
			}
			if _, _, err := tmplayer.Read(&out, model.CommonReadOptions{}, tmplayer.ReadOptions{}); err != nil {
				t.Fatalf("Read of the written output returned %v", err)
			}
		}
	})
}
