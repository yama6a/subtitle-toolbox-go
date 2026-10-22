package stbox_test

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/markup"
)

const benchCues = 20000

// BenchmarkParse ports tests/Parsers/ParseSpeedTest.php of subtitle-toolbox-php: 20,000 cues of
// srt/real/own_escaping.srt, copy k shifted by k periods, written once in each format.
func BenchmarkParse(b *testing.B) {
	base, _, err := stbox.Load("testdata/srt/real/own_escaping.srt", stbox.ReadOptions{Format: stbox.SubRip})
	if err != nil {
		b.Fatal(err)
	}
	last, _ := base.Cue(base.Len() - 1)
	period := last.End.Truncate(time.Second) + time.Second
	if last.End%time.Second != 0 {
		period += time.Second
	}
	s := &stbox.Subtitle{}
	cues := make([]stbox.Cue, 0, benchCues)
	for k := 0; len(cues) < benchCues; k++ {
		for _, c := range base.All() {
			if len(cues) == benchCues {
				break
			}
			c = c.Clone()
			c.Start += time.Duration(k) * period
			c.End += time.Duration(k) * period
			cues = append(cues, c)
		}
	}
	s.AddCues(cues...)

	for _, f := range []stbox.Format{stbox.SubRip, stbox.TTML, stbox.WebVTT, stbox.ASS} {
		var content bytes.Buffer
		if f == stbox.TTML {
			content.WriteString(ttmlDocument(s))
		} else if err := stbox.Write(&content, s, f, stbox.WriteOptions{}); err != nil {
			b.Fatal(err)
		}
		b.Run(string(f), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := stbox.Parse(bytes.NewReader(content.Bytes()), stbox.ReadOptions{Format: f}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ttmlDocument writes the cues of s as TTML paragraphs with plain text, because the library has no TTML writer yet.
func ttmlDocument(s *stbox.Subtitle) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<tt xmlns="http://www.w3.org/ns/ttml"><body><div>` + "\n")
	for _, c := range s.All() {
		lines := make([]string, len(c.Lines))
		for i, line := range c.Lines {
			var text bytes.Buffer
			_ = xml.EscapeText(&text, []byte(markup.PlainText(line)))
			lines[i] = text.String()
		}
		fmt.Fprintf(&b, "<p begin=\"%.3fs\" end=\"%.3fs\">%s</p>\n", c.Start.Seconds(), c.End.Seconds(), strings.Join(lines, "<br/>"))
	}
	b.WriteString("</div></body></tt>\n")
	return b.String()
}
