package srt_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	content := "1\n00:00:01,500 --> 00:00:04,000\n<i>Fish & Chips</i>\n\n" +
		"2\n00:00:05,000 --> 00:00:06,000 X1:10 X2:20 Y1:30 Y2:40\n{\\an8}On the menu\n"
	s, warnings, err := srt.Read(strings.NewReader(content), model.CommonReadOptions{}, srt.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines, c.Alignment)
	}
	fmt.Println(len(warnings), "warnings")
	// Output:
	// 1.5s 4s [<i>Fish &amp; Chips</i>] BottomCenter
	// 5s 6s [On the menu] TopCenter
	// 0 warnings
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"<i>Fish &amp; Chips</i>"}},
		model.Cue{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"On the menu"}, Alignment: model.TopCenter},
	)
	err := srt.Write(os.Stdout, s, model.CommonWriteOptions{BOM: model.BOMRemove}, srt.WriteOptions{})
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
	// {\an8}On the menu
}

func ExampleDetect() {
	fmt.Println(srt.Detect([]byte("1\n00:00:01,500 --> 00:00:04,000\nHello\n")))
	fmt.Println(srt.Detect([]byte("WEBVTT\n\n00:00:01.500 --> 00:00:04.000\nHello\n")))
	// Output:
	// true
	// false
}
