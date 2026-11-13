package stbox_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
)

func ExampleLoad() {
	s, info, err := stbox.Load("testdata/srt/real/own_missing_empty_line.srt", stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(info.Format, info.SourceEncoding, s.Len(), "cues")
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// srt UTF-8 4 cues
	// 1s 2.5s The bus to the airport is full.
	// 3s 5s The next one leaves at four.
	// 5.5s 7s Tickets are on sale inside.
	// 8s 10s Mind the step.
}

func ExampleParse() {
	content := "1\n00:00:01,500 --> 00:00:04,000\n<i>Fish & Chips</i>\n\n" +
		"2\n00:00:05,000 --> 00:00:06,000\nAT&amp T\n\n" +
		"3\n00:00:07,000 --> 00:00:0"
	r := strings.NewReader(content)
	s, info, err := stbox.Parse(r, r.Size(), stbox.ReadOptions{Lenient: true})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	for _, w := range info.Warnings {
		fmt.Println(w.Action, "line", w.Line, w.Message)
	}
	// Output:
	// 1.5s 4s [<i>Fish &amp; Chips</i>]
	// 5s 6s [AT&amp;amp T]
	// skipped line 10 The time "00:00:0" is not valid.
}

func ExampleWrite() {
	s := &stbox.Subtitle{}
	s.AddCues(
		stbox.Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"<i>Fish &amp; Chips</i>"}},
		stbox.Cue{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"<c.yellow>Now</c> open"}, Position: stbox.Position{Alignment: stbox.TopCenter}},
	)
	_, err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// 1
	// 00:00:01,500 --> 00:00:04,000
	// <i>Fish & Chips</i>
	//
	// 2
	// 00:00:05,000 --> 00:00:06,000
	// {\an8}<font color="#ffff00">Now</font> open
}

func ExampleDetect() {
	for _, content := range []string{
		"\ufeffWEBVTT\r\n\r\n00:01.000 --> 00:02.000\r\nRain\r\n",
		"1\n00:00:01,000 --> 00:00:04,000\nHello\n",
		`{"version": 1, "cues": []}`,
		"Hello\n",
	} {
		d, ok := stbox.Detect([]byte(content))
		f := d.Format
		fmt.Printf("%q %t\n", f, ok)
	}
	// Output:
	// "vtt" true
	// "srt" true
	// "json" true
	// "" false
}

func ExampleTracks() {
	b, err := os.ReadFile("testdata/mkv/mkvmerge.mkv")
	if err != nil {
		fmt.Println(err)
		return
	}
	tracks, err := stbox.Tracks(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, t := range tracks {
		fmt.Println(t.Container, t.Number, t.CodecID, t.Format)
	}
	// Output:
	// matroska 2 S_TEXT/UTF8 srt
	// matroska 3 S_TEXT/ASS ass
}
