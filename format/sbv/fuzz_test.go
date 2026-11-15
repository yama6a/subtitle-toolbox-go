package sbv_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/sbv"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "sbv/*.sbv", "sbv/real/*.sbv", "lenient/*.sbv")
	for _, seed := range []string{
		"0:00:01.000,0:00:02.000",
		"0:00:01.000,0:00:0",
		"99999999999999999999:00:04.0.0:00:06.0\nText",
		"0:00:07.9995 , 0:0:11,3\nA\n0:00:12.000,0:00:13.000\nB\n\nC",
		"0:00:05.000,0:00:04.000\r\r\nA\r\n\r\n1:2\nB &amp; <i>c\xe9\x00\x01",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := sbv.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, sbv.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			if _, err := sbv.Write(&out, s, model.CommonWriteOptions{}, sbv.WriteOptions{}); err != nil {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}
