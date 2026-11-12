package ebustl_test

import (
	"bytes"
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func ExampleRead() {
	file := append(gsiBlock(nil), ttiBlock(tti{text: "\x80Caf\xC2e\x81\x8A"})...)
	s, _, err := ebustl.Read(bytes.NewReader(file), model.CommonReadOptions{}, ebustl.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	// Output:
	// 1.2s 2.4s [<i>Café</i>]
}

func ExampleWrite() {
	s := &model.Subtitle{}
	c := model.Cue{Start: time.Second + 500*time.Millisecond, End: 2*time.Second + 480*time.Millisecond}
	c.AddLine("Hello")
	s.AddCue(c)
	var out bytes.Buffer
	report, err := ebustl.Write(&out, s, model.CommonWriteOptions{}, ebustl.WriteOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(report.CharacterCodeTable, out.Len())
	fmt.Printf("% X\n", out.Bytes()[1024:1024+21])
	// Output:
	// 00 1152
	// 00 01 00 FF 00 00 00 01 0D 00 00 02 0C 17 02 00 48 65 6C 6C 6F
}
