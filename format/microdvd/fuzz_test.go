package microdvd_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "microdvd/*.sub", "microdvd/real/*.sub", "lenient/*.sub", "cli/*.sub")
	for _, seed := range []string{
		"{1}{1}0\n{25}{50}Hello",
		"{1}{1}Hello",
		"{24}{}Hello",
		"{24}{99999999999999999999}Hello",
		"One|{Y:u}Two",
		"{0}{25}{C:$0000FF}{Y:i}One|{c:$00FF00}{y:b}Two",
		"{0}{25}{y:i}caf\xe9 & bread|<3 jam",
		"{359999999999650}{359999999999650}x",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := microdvd.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, microdvd.ReadOptions{FrameRate: model.FrameRate{Num: 25, Den: 1}})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var opts microdvd.WriteOptions
			if _, ok := s.FormatData("microdvd"); !ok {
				opts.FrameRate = model.FrameRate{Num: 25, Den: 1} // a blank input gives no frame rate to write with
			}
			var out bytes.Buffer
			if _, err := microdvd.Write(&out, s, model.CommonWriteOptions{}, opts); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}
