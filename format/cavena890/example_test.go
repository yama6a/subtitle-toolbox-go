package cavena890_test

import (
	"bytes"
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/cavena890"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"<i>Grüße</i> [x]"}})

	var out bytes.Buffer
	if _, err := cavena890.Write(&out, s, model.CommonWriteOptions{}, cavena890.WriteOptions{}); err != nil {
		fmt.Println(err)
		return
	}
	back, _, err := cavena890.Read(&out, model.CommonReadOptions{}, cavena890.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range back.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	// Output:
	// 1s 4s [<i>Grüße</i> [x]]
}
