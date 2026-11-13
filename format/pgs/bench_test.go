package pgs_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// BenchmarkRead replaces a heap limit test. `make bench` prints the allocated bytes of a full 4K object.
func BenchmarkRead(b *testing.B) {
	for _, size := range []struct{ w, h int }{{1920, 1080}, {3840, 2160}} {
		pixels := append(bytes.Repeat([]byte{1}, size.w*size.h/2), bytes.Repeat([]byte{3}, size.w*size.h/2)...)
		content := new(builder).shows(90000, size.w, size.h, obj{id: 0, w: size.w, h: size.h, pixels: string(pixels)}).Bytes()
		b.Run(fmt.Sprintf("size=%dx%d", size.w, size.h), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := pgs.Read(bytes.NewReader(content), model.CommonReadOptions{}, pgs.ReadOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkWrite replaces a heap limit test. It writes one 4K indexed image of 2 colours.
func BenchmarkWrite(b *testing.B) {
	pixels := append(bytes.Repeat([]byte{1}, 3840*2160/2), bytes.Repeat([]byte{3}, 3840*2160/2)...)
	content := new(builder).shows(90000, 3840, 2160, obj{id: 0, w: 3840, h: 2160, pixels: string(pixels)}).Bytes()
	s, _, err := pgs.Read(bytes.NewReader(content), model.CommonReadOptions{}, pgs.ReadOptions{})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := pgs.Write(io.Discard, s, model.CommonWriteOptions{}, pgs.WriteOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}
