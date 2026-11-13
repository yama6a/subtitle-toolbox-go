package pac_test

import (
	"bytes"
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleRead() {
	content := file("Fresh rolls are<warm>today.")
	s, _, err := pac.Read(bytes.NewReader(content), model.CommonReadOptions{}, pac.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Position.Alignment, c.Lines)
	}
	// Output:
	// 1s 2s BottomCenter [Fresh rolls are <i>warm</i> today.]
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{End: 1500000000, Lines: []string{"Fresh <i>rolls</i>"}})
	var out bytes.Buffer
	if _, err := pac.Write(&out, s, model.CommonWriteOptions{}, pac.WriteOptions{}); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%q\n", out.Bytes()[20:])
	// Output:
	// "\x00\x00\x00`\x00\x00\x00\x00\x00\x00q\x00\x0f\x00\v\xfe\x02\x03Fresh<rolls\xff\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x11\x00dummy end of file"
}
