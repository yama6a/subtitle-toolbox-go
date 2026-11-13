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

// srv3Windows has benchCaptions paragraphs in 50 windows, each with its own start and no duration.
func srv3Windows() string {
	var b strings.Builder
	b.WriteString(`<timedtext format="3"><body>`)
	for i := range benchCaptions {
		fmt.Fprintf(&b, `<p t="%d" w="%d">caption number %d with some words</p>`, i*10, i%50, i)
	}
	return b.String() + "</body></timedtext>"
}

// srv3OneParagraph is one paragraph with benchCaptions timed segments of one word each.
func srv3OneParagraph() string {
	var b strings.Builder
	b.WriteString(`<timedtext format="3"><body><p t="0" d="999999">`)
	for i := range benchCaptions {
		fmt.Fprintf(&b, `<s t="%d" ac="0"> word%d</s>`, i*10, i)
	}
	return b.String() + "</p></body></timedtext>"
}

func BenchmarkRead(b *testing.B) {
	for _, bm := range []struct{ name, content string }{
		{fmt.Sprintf("srv1 captions=%d", benchCaptions), srv1Without()},
		{fmt.Sprintf("srv3 windows captions=%d", benchCaptions), srv3Windows()},
		{fmt.Sprintf("srv3 segments=%d", benchCaptions), srv3OneParagraph()},
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
	for _, bm := range []struct{ name, content string }{
		{fmt.Sprintf("srv1 captions=%d", benchCaptions), srv1Without()},
		{fmt.Sprintf("srv3 windows captions=%d", benchCaptions), srv3Windows()},
		{fmt.Sprintf("srv3 segments=%d", benchCaptions), srv3OneParagraph()},
	} {
		s, _, err := youtube.Read(strings.NewReader(bm.content), model.CommonReadOptions{}, withWords)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bm.content)))
			for b.Loop() {
				if err := youtube.Write(io.Discard, s, model.CommonWriteOptions{}, youtube.WriteOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkWriteChanged writes a cue whose text changed, so the writer matches each new segment to a stored one.
func BenchmarkWriteChanged(b *testing.B) {
	content := srv3OneParagraph()
	s, _, err := youtube.Read(strings.NewReader(content), model.CommonReadOptions{}, withWords)
	if err != nil {
		b.Fatal(err)
	}
	c, _ := s.Cue(0)
	c.Words[len(c.Words)-1].Text = "changed"
	c.Lines[0] = strings.Replace(c.Lines[0], fmt.Sprintf("word%d", benchCaptions-1), "changed", 1)
	if err := s.SetCue(0, c); err != nil {
		b.Fatal(err)
	}
	b.Run(fmt.Sprintf("srv3 segments=%d", benchCaptions), func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(content)))
		for b.Loop() {
			if err := youtube.Write(io.Discard, s, model.CommonWriteOptions{}, youtube.WriteOptions{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
