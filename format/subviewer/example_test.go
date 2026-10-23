package subviewer_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	input := "[TITLE]\nWeather report\n[DELAY]\n2\n******** START SCRIPT ********\n" +
		"[00:00:01]\nSunny|cloudy\n[00:00:04]\n\n[00:00:06]\nLight wind\n[00:00:09]\nSnow tonight\n"
	s, warnings, err := subviewer.Read(strings.NewReader(input), model.CommonReadOptions{}, subviewer.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	title, _ := s.Metadata(model.MetadataTitle)
	fmt.Println(title)
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	fmt.Println(len(warnings), "warnings")
	// Output:
	// Weather report
	// 3s 6s [Sunny cloudy]
	// 8s 11s [Light wind]
	// 11s 16s [Snow tonight]
	// 0 warnings
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"First", "line"}},
		model.Cue{Start: 5 * time.Second, End: 7400 * time.Millisecond, Lines: []string{"<i>Second</i>"}},
	)
	opts := subviewer.WriteOptions{Version: subviewer.V1}
	if err := subviewer.Write(os.Stdout, s, model.CommonWriteOptions{}, opts); err != nil {
		fmt.Println(err)
	}
	// Output:
	// [TITLE]
	// [AUTHOR]
	// [SOURCE]
	// [PRG]
	// [FILEPATH]
	// [DELAY]
	// 0
	// [CD TRACK]
	// 0
	// [BEGIN]
	// ******** START SCRIPT ********
	// [00:00:02]
	// First|line
	// [00:00:04]
	//
	// [00:00:05]
	// Second
	// [00:00:07]
	//
	// [end]
	// ******** END SCRIPT ********
}

func ExampleWriteOptions_Validate() {
	fmt.Println(subviewer.WriteOptions{Version: subviewer.V1}.Validate())
	fmt.Println(subviewer.WriteOptions{Version: subviewer.Version(3)}.Validate())
	// Output:
	// <nil>
	// stbox: invalid argument: WriteOptions.SubViewer.Version: The SubViewer version must be 1 or 2, got 3.
}

func ExampleDetect() {
	fmt.Println(subviewer.Detect([]byte("[INFORMATION]\n[TITLE]Weather report\n")))
	fmt.Println(subviewer.Detect([]byte("[00:00:01]\nSunny\n")))
	// Output:
	// true
	// false
}
