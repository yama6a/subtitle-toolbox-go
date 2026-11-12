package pgs_test

import (
	"bytes"
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func ExampleRead() {
	var b builder
	b.shows(90000, 1920, 1080, obj{id: 0, x: 640, y: 940, w: 4, h: 1, pixels: "\x01\x01\x03\x03"})
	b.clears(315000, 1920, 1080)

	s, _, err := pgs.Read(bytes.NewReader(b.Bytes()), model.CommonReadOptions{}, pgs.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Image.X, c.Image.Y, c.Image.Width, c.Image.Height)
	}
	// Output:
	// 1s 3.5s 640 940 4 1
}
