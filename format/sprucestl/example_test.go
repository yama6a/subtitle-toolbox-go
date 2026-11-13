package sprucestl_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/sprucestl"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleRead() {
	content := "$Italic = False\n00:02:17:10 , 00:02:20:10 , The bakery opens at seven, | every day of the week.\n"
	s, _, err := sprucestl.Read(strings.NewReader(content), model.CommonReadOptions{}, sprucestl.ReadOptions{FrameRate: model.FrameRate{Num: 25, Den: 1}})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	// Output:
	// 2m17.4s 2m20.4s [The bakery opens at seven, every day of the week.]
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: 137400 * time.Millisecond, End: 140360 * time.Millisecond, Lines: []string{"The bakery opens at seven,", "<i>every day</i> of the week."}})
	if _, err := sprucestl.Write(os.Stdout, s, model.CommonWriteOptions{}, sprucestl.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// $FontName = Arial
	// $FontSize = 42
	// $HorzAlign = Center
	// $VertAlign = Bottom
	// $XOffset = 0
	// $YOffset = 0
	// $Bold = False
	// $Italic = False
	// $Underlined = False
	// $TextContrast = 15
	// $Outline1Contrast = 15
	// $Outline2Contrast = 15
	// $BackgroundContrast = 0
	// $ForceDisplay = False
	// $FadeIn = 0
	// $FadeOut = 0
	// $TapeOffset = False
	//
	// 00:02:17:10 , 00:02:20:09 , The bakery opens at seven,|^Ievery day^I of the week.
}
