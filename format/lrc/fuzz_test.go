package lrc_test

import (
	"bytes"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/lrc"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "lrc/*.lrc", "lrc/real/*.lrc", "karaoke/*.lrc", "lenient/*.lrc")
	for _, seed := range []string{
		"[00:12.00][01:15.30]Chorus\n[00:17.20]Verse\n",
		"[00:01.00] <00:01.00> Fish <00:01.50> chips <00:02.00>\n",
		"[00:01.00]<00:01>Fish & <00:01.50>chips <3 <1:2>\n",
		"[offset:-99999999999999999999]\n[00:12.00]x\n",
		"[offset:-359999987999]\n[00:12.00]x <00:12.00>y\n",
		"[00:02:35]\nHello\n[#:note]\n[ti:x]\n[00:1x.00]y\n",
		"[ti:Harbour walk]\nThe ferry leaves at nine.\n",
		"[00:01.00]caf\xE9\x00\x0b & tea\r\r\n\t[ 1:2:03.5 ]",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := lrc.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, lrc.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			if _, err := lrc.Write(&out, s, model.CommonWriteOptions{}, lrc.WriteOptions{}); err != nil {
				t.Fatalf("Write returned %v", err)
			}
			if _, _, err := lrc.Read(&out, model.CommonReadOptions{}, lrc.ReadOptions{}); err != nil {
				t.Fatalf("Read of the written output returned %v", err)
			}
		}
	})
}
