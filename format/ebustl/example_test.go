package ebustl_test

import (
	"bytes"
	"fmt"

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
