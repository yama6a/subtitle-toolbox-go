package webvtt_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	const content = `WEBVTT
Kind: captions

NOTE Morning show

intro
00:00:01.000 --> 00:00:04.000 line:0 align:left
<v Anna>Good morning.
`
	s, _, err := webvtt.Read(strings.NewReader(content), model.CommonReadOptions{}, webvtt.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	c, _ := s.Cue(0)
	fmt.Println(c.ID, c.Start, c.End, c.Alignment, c.Text())
	fmt.Println(s.Comments()[0].Text)
	file, _ := s.FormatData("vtt")
	headerLines, _ := file.Strings("headerLines")
	fmt.Println(headerLines)
	// Output:
	// intro 1s 4s TopLeft <v Anna>Good morning.
	// Morning show
	// [Kind: captions]
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"<v Anna>Good morning."}, Alignment: model.TopLeft})
	if err := s.AddComment("Morning show", 0); err != nil {
		fmt.Println(err)
		return
	}
	err := webvtt.Write(os.Stdout, s, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// WEBVTT
	//
	// NOTE Morning show
	//
	// 1
	// 00:00:01.000 --> 00:00:04.000 line:0 align:left
	// <v Anna>Good morning.
}

func ExampleWriteWithReport() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{`<font color="#123456">Rain</font> <font color="#ffff00">and sun</font>`}})
	var out strings.Builder
	report, err := webvtt.WriteWithReport(&out, s, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print(out.String())
	for _, d := range report.DroppedColors {
		fmt.Println(d.Message)
	}
	// Output:
	// WEBVTT
	//
	// 1
	// 00:00:04.000 --> 00:00:06.000
	// Rain <c.yellow>and sun</c>
	// Cue #0 at 4 s: dropped the color "#123456", because WebVTT has classes for 8 colors only.
}
