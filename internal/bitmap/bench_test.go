package bitmap_test

import (
	"fmt"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/bitmap"
)

// BenchmarkReduce replaces a heap limit test. `make bench` prints the allocated bytes of each size.
func BenchmarkReduce(b *testing.B) {
	for _, size := range []struct{ w, h, colors int }{{1920, 1080, 20000}, {3840, 2160, 300}} {
		pixels := make([]uint32, size.w*size.h)
		for i := range pixels {
			pixels[i] = uint32(i*7%size.colors)*0x00010305 | 0xFF
		}
		b.Run(fmt.Sprintf("size=%dx%d,colors=%d", size.w, size.h, size.colors), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := bitmap.Reduce(pixels, 256); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkImageOf(b *testing.B) {
	parts := make([]bitmap.Part, 2)
	for i := range parts {
		parts[i] = bitmap.Part{X: 0, Y: 100 + 300*i, Width: 1920, Height: 200, Indexes: make([]byte, 1920*200)}
		for j := range parts[i].Indexes {
			parts[i].Indexes[j] = byte(j % 200)
		}
		for j := range parts[i].Palette {
			parts[i].Palette[j] = uint32(j)*0x01010101 | 0xFF
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := bitmap.ImageOf(parts, 1920, 1080); err != nil {
			b.Fatal(err)
		}
	}
}
