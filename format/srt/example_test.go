package srt_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
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
		fmt.Println(c.Start, c.End, c.Lines, c.Position.Alignment)
	}
	fmt.Println(len(warnings), "warnings")
	// Output:
	// 1.5s 4s [<i>Fish &amp; Chips</i>] BottomCenter
	// 5s 6s [On the menu] TopCenter
	// 0 warnings
}

func ExampleWrite() {
	s := &model.Subtitle{}
	if err := s.AddCues(
		model.Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"<i>Fish &amp; Chips</i>"}},
		model.Cue{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"On the menu"}, Position: model.Position{Alignment: model.TopCenter}},
	); err != nil {
		panic(err)
	}
	_, err := srt.Write(os.Stdout, s, model.CommonWriteOptions{BOM: model.BOMRemove}, srt.WriteOptions{})
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

func ExampleStreamReader() {
	input := "1\n00:00:01,000 --> 00:00:02,500\nThe ferry leaves.\n\n2\n00:00:04,000 --> 00:00:05,000\nIt is <i>late</i>.\n"

	w, err := webvtt.NewStreamWriter(os.Stdout, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{}, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	for c, err := range srt.NewStreamReader(strings.NewReader(input), srt.StreamOptions{}).Cues() {
		if err != nil {
			fmt.Println(err)
			return
		}
		c.Start += 2 * time.Second
		c.End += 2 * time.Second
		if err := w.Write(c); err != nil {
			fmt.Println(err)
			return
		}
	}
	if err := w.Close(); err != nil {
		fmt.Println(err)
	}
	// Output:
	// WEBVTT
	//
	// 1
	// 00:00:03.000 --> 00:00:04.500
	// The ferry leaves.
	//
	// 2
	// 00:00:06.000 --> 00:00:07.000
	// It is <i>late</i>.
}

func ExampleStreamWriter() {
	w, err := srt.NewStreamWriter(os.Stdout, model.CommonWriteOptions{BOM: model.BOMRemove}, srt.WriteOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, text := range []string{"The ferry leaves.", "It is late."} {
		var c model.Cue
		c.SetText(text)
		c.End = time.Second
		if err := w.Write(c); err != nil {
			fmt.Println(err)
			return
		}
	}
	// Output:
	// 1
	// 00:00:00,000 --> 00:00:01,000
	// The ferry leaves.
	//
	// 2
	// 00:00:00,000 --> 00:00:01,000
	// It is late.
}
