package webvtt_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
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
	fmt.Println(c.ID, c.Start, c.End, c.Position.Alignment, taggedText(c))
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
	if err := s.AddCue(model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"<v Anna>Good morning."}, Position: model.Position{Alignment: model.TopLeft}}); err != nil {
		panic(err)
	}
	if err := s.AddComment("Morning show", 0); err != nil {
		fmt.Println(err)
		return
	}
	_, err := webvtt.Write(os.Stdout, s, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{})
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

func ExampleWrite_droppedColors() {
	s := &model.Subtitle{}
	if err := s.AddCue(model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{`<font color="#123456">Rain</font> <font color="#ffff00">and sun</font>`}}); err != nil {
		panic(err)
	}
	var out strings.Builder
	report, err := webvtt.Write(&out, s, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{})
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

func ExampleDetect() {
	fmt.Println(webvtt.Detect([]byte("WEBVTT\n\n00:00:01.500 --> 00:00:04.000\nHello\n")))
	fmt.Println(webvtt.Detect([]byte("WEBVTTX\n\n00:00:01.500 --> 00:00:04.000\nHello\n")))
	fmt.Println(webvtt.Detect([]byte("1\n00:00:01,500 --> 00:00:04,000\nHello\n")))
	// Output:
	// true
	// false
	// false
}

func ExampleStreamReader() {
	input := "WEBVTT Harbor walk\n\nSTYLE\n::cue { color: yellow }\n\n00:01.000 --> 00:02.000\nThe ferry leaves.\n"

	rd := webvtt.NewStreamReader(strings.NewReader(input), webvtt.StreamOptions{})
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

func ExampleStreamReader_utf16() {
	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	input, err := utf16.NewEncoder().String("WEBVTT\n\n00:01.000 --> 00:02.000\nThe ferry leaves.\n")
	if err != nil {
		fmt.Println(err)
		return
	}

	r := transform.NewReader(strings.NewReader(input), utf16.NewDecoder())
	for c, err := range webvtt.NewStreamReader(r, webvtt.StreamOptions{}).Cues() {
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(c.Start, c.Text())
	}
	// Output:
	// 1s The ferry leaves.
}

func ExampleStreamWriter() {
	w, err := webvtt.NewStreamWriter(os.Stdout, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{}, nil)
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
