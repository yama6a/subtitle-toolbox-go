package ass_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "ass/*.ass", "ass/real/*.ass", "ass/real/*.ssa", "lenient/*.ass", "editing/*.ass", "quotes-in-tags/*.ass", "ruby/*.ass", "encoding/*.ass")
	for _, seed := range []string{
		eventsHeader + `Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\b1}a{\i1}b{\b0}c{\t(\1c&HFF&)}{\p1}m{\p0}\N\h`,
		eventsHeader + `Dialogue: 0,99999:59:59.00,99999:59:59.50,Default,,0,0,0,,{\k50}a{\k50}b`,
		eventsHeader + "Dialogue: 0,0:00:07,50,0:00:08,00,Default,,0,0,0,,a\nComment: 0,0:00:01.00,0:00:01.00,Default,,0,0,0,,n",
		"[Events]\nFormat: Start, End, 1, Text\nDialogue: 0:00:01.00,0:00:02.00,x,a\nhello",
		"[V4 Styles]\nStyle: Default,Arial,1,0,0,0,0,-1,1,1,1,0,10\n[Events]\nDialogue: Marked=0,0:00:01.00,0:00:00.5,Default,,0,0,0,,{\\a10\\rDefault}\xe9\x00\x01",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := ass.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, ass.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			if _, err := ass.Write(&out, s, model.CommonWriteOptions{}, ass.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}
