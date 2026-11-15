package sami_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "sami/real/*.smi", "lenient/*.smi")
	for _, seed := range []string{
		"<SAMI><BODY><SYNC Start=0><P Class=ENCC>one<P Class=FRCC>un\n<SYNC Start=2000>&nbsp;\n</BODY></SAMI>",
		"<SYNC Start=-200><P>a&nbsp<SYNC Start=99999999999999999999><P>b<SYNC End=1><P>c",
		"<SAMI><HEAD><STYLE>.A{lang:x}</STYLE></HEAD><BODY><SYNC Start=1><P Class=B>b</BODY>",
		"<SAMI>\n<HEAD><TITLE>Walk</TITLE></HEAD>\n<BODY>\n<P>The ferry leaves.</P>\n</BODY>\n</SAMI>\n",
		"<SYNC Start=0><P Class=ENCC><b>open <i>tags</p></div></font><font color=red>x\xff\x00",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := sami.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, sami.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			if _, err := sami.Write(&out, s, model.CommonWriteOptions{}, sami.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}
