package stbox_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go"
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
	s, info, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{Lenient: true})
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
		stbox.Cue{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"<c.yellow>Now</c> open"}, Alignment: stbox.TopCenter},
	)
	err := stbox.Write(os.Stdout, s, stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove})
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
