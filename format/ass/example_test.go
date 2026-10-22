package ass_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	content := `[V4+ Styles]
Format: Name, Italic, Alignment
Style: Thoughts,-1,2

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.50,0:00:04.00,Thoughts,Anna,0,0,0,,{\an8}Not {\b1}again{\b0}
`
	s, _, err := ass.Read(strings.NewReader(content), model.CommonReadOptions{}, ass.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Alignment, c.Lines)
	}
	// Output:
	// 1.5s 4s TopCenter [<v Anna><i>Not <b>again</b></i>]
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"<i>Fish &amp; Chips</i>", "today"}},
		model.Cue{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"On the menu"}, Alignment: model.TopCenter},
	)
	err := ass.Write(os.Stdout, s, model.CommonWriteOptions{BOM: model.BOMRemove}, ass.WriteOptions{})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// [Script Info]
	// ScriptType: v4.00+
	// PlayResX: 384
	// PlayResY: 288
	// ScaledBorderAndShadow: yes
	//
	// [V4+ Styles]
	// Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
	// Style: Default,Arial,16,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,1,0,2,10,10,10,1
	//
	// [Events]
	// Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
	// Dialogue: 0,0:00:01.50,0:00:04.00,Default,,0,0,0,,{\i1}Fish & Chips{\i0}\Ntoday
	// Dialogue: 0,0:00:05.00,0:00:06.00,Default,,0,0,0,,{\an8}On the menu
}

func ExampleWriteOptions() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: 10 * time.Second, End: 12 * time.Second, Lines: []string{"Ka<00:00:10.300>ra"}})
	opts := ass.WriteOptions{KaraokeTag: ass.KaraokeFill, Style: "Fontname=Roboto,Fontsize=48"}
	var out strings.Builder
	if err := ass.Write(&out, s, model.CommonWriteOptions{}, opts); err != nil {
		fmt.Println(err)
		return
	}
	for line := range strings.SplitSeq(out.String(), "\n") {
		if strings.HasPrefix(line, "Style:") || strings.HasPrefix(line, "Dialogue:") {
			fmt.Println(line)
		}
	}
	// Output:
	// Style: Default,Roboto,48,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,1,0,2,10,10,10,1
	// Dialogue: 0,0:00:10.00,0:00:12.00,Default,,0,0,0,,{\kf30}Ka{\kf170}ra
}

func ExampleDetect() {
	fmt.Println(ass.Detect([]byte("[Script Info]\nTitle: Bakery\n")))
	fmt.Println(ass.Detect([]byte("1\n00:00:01,500 --> 00:00:04,000\nHello\n")))
	// Output:
	// true
	// false
}
