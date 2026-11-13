package youtube_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// The inputs hold about 1 MB. The benchmarks have no time limit, so a rise from linear to quadratic cost shows in
// the numbers of the bench run.
const benchCaptions = 20000

// srv1Without has benchCaptions captions without a duration and the same start, so each end needs the next later start.
func srv1Without() string {
	var b strings.Builder
	b.WriteString("<transcript>")
	for i := range benchCaptions {
		fmt.Fprintf(&b, `<text start="%d">caption number %d with some more words</text>`, i/100, i)
	}
	return b.String() + "</transcript>"
}

func BenchmarkRead(b *testing.B) {
	for _, bm := range []struct{ name, content string }{
		{fmt.Sprintf("srv1 captions=%d", benchCaptions), srv1Without()},
	} {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bm.content)))
			for b.Loop() {
				if _, _, err := youtube.Read(strings.NewReader(bm.content), model.CommonReadOptions{}, withWords); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkWrite(b *testing.B) {
	content := srv1Without()
	s, _, err := youtube.Read(strings.NewReader(content), model.CommonReadOptions{}, withWords)
	if err != nil {
		b.Fatal(err)
	}
	b.Run(fmt.Sprintf("srv1 captions=%d", benchCaptions), func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(content)))
		for b.Loop() {
			if err := youtube.Write(io.Discard, s, model.CommonWriteOptions{}, youtube.WriteOptions{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
