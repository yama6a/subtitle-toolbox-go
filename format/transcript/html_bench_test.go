package transcript_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const htmlBenchParagraphs = 12000

// htmlParagraphs is about 1 MB of paragraphs with a speaker, a time and a p each.
func htmlParagraphs() string {
	var b strings.Builder
	for i := range htmlBenchParagraphs {
		fmt.Fprintf(&b, "<cite>Speaker %d:</cite>\n<time>%d:%02d</time>\n<p>Words of the paragraph %d<br>and a second line.</p>\n", i%3, i/60, i%60, i)
	}
	return b.String()
}

func BenchmarkReadHTML(b *testing.B) {
	for _, bm := range []struct{ name, content string }{
		{fmt.Sprintf("paragraphs=%d", htmlBenchParagraphs), htmlParagraphs()},
		{"unclosed p=170000", strings.Repeat("<p>unclosed ", 85000) + strings.Repeat("<p class=", 85000)},
		{"unclosed cite and time=170000", strings.Repeat("<cite>a<time>b", 85000) + strings.Repeat("<p>", 85000)},
	} {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bm.content)))
			for b.Loop() {
				if _, _, err := transcript.ReadHTML(strings.NewReader(bm.content), model.CommonReadOptions{}, transcript.ReadOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkWriteHTML(b *testing.B) {
	s, _, err := transcript.ReadHTML(strings.NewReader(htmlParagraphs()), model.CommonReadOptions{}, transcript.ReadOptions{})
	if err != nil {
		b.Fatal(err)
	}
	b.Run(fmt.Sprintf("paragraphs=%d", htmlBenchParagraphs), func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := transcript.WriteHTML(io.Discard, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
