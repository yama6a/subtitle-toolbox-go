package stream_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/stream"
)

func ExampleSubRipReader() {
	input := "1\n00:00:01,000 --> 00:00:02,500\nThe ferry leaves.\n\n2\n00:00:04,000 --> 00:00:05,000\nIt is <i>late</i>.\n"

	w, err := stream.NewWebVTTWriter(os.Stdout, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{}, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	for c, err := range stream.NewSubRipReader(strings.NewReader(input), stream.ReadOptions{}).Cues() {
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

func ExampleWebVTTReader() {
	input := "WEBVTT Harbor walk\n\nSTYLE\n::cue { color: yellow }\n\n00:01.000 --> 00:02.000\nThe ferry leaves.\n"

	rd := stream.NewWebVTTReader(strings.NewReader(input), stream.ReadOptions{})
	for c, err := range rd.Cues() {
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(c.Text())
	}
	title, _ := rd.Header().String("header")
	styles, _ := rd.Header().Strings("styles")
	fmt.Println(title, styles)
	// Output:
	// The ferry leaves.
	// Harbor walk [::cue { color: yellow }]
}

func ExampleWebVTTReader_utf16() {
	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	input, err := utf16.NewEncoder().String("WEBVTT\n\n00:01.000 --> 00:02.000\nThe ferry leaves.\n")
	if err != nil {
		fmt.Println(err)
		return
	}

	r := transform.NewReader(strings.NewReader(input), utf16.NewDecoder())
	for c, err := range stream.NewWebVTTReader(r, stream.ReadOptions{}).Cues() {
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(c.Start, c.Text())
	}
	// Output:
	// 1s The ferry leaves.
}

func ExampleSubRipWriter() {
	w, err := stream.NewSubRipWriter(os.Stdout, model.CommonWriteOptions{BOM: model.BOMRemove}, srt.WriteOptions{})
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

func ExampleWebVTTWriter() {
	w, err := stream.NewWebVTTWriter(os.Stdout, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{}, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	c := model.Cue{End: time.Second}
	c.SetText(`<font color="#123456">Hi</font>`)
	if err := w.Write(c); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(w.DroppedColors()[0].Message)
	// Output:
	// WEBVTT
	//
	// 1
	// 00:00:00.000 --> 00:00:01.000
	// Hi
	// Cue #0 at 0 s: dropped the color "#123456", because WebVTT has classes for 8 colors only.
}
