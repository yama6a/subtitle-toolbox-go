package ttml_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "ttml/real/*", "forced/*", "lenient/*")
	for _, seed := range []string{
		document(`<div begin="10s" end="20s"><p begin="1s" end="2s" dur="5s">a<span begin="0.5s">b</span></p><p begin="30s">c</p></div>`, "", ""),
		document(`<div><p begin="00:00:01:45" end="99999:59:59.9999">a&nbsp;&#13;</p><p begin="5s" end="2s">x</p></div>`,
			`ttp:timeBase="smpte" ttp:frameRate="-0.001" ttp:dropMode="dropNTSC"`, ""),
		document(`<div region="r"><p end="1s" style="a" ttm:agent="x">x<br>y</p>`, "", `<styling><style xml:id="a" style="a"/></styling>`),
		"Credit\n<?xml version=\"1.0\"?><tt><body><div><p begin=\"1\" dur=\"1:2\">a & b</div></body></tt>",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := ttml.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, ttml.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			for i, c := range s.All() {
				if err := c.Position.Validate(); err != nil {
					t.Fatalf("cue %d has the position %+v: %v", i, c.Position, err)
				}
			}
			var out bytes.Buffer
			if _, err := ttml.Write(&out, s, model.CommonWriteOptions{}, ttml.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
				t.Fatalf("Write returned %v", err)
			}
		}
	})
}
