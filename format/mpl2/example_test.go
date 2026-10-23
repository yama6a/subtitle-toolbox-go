package mpl2_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/mpl2"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	s, warnings, err := mpl2.Read(strings.NewReader("[12][45]Where are you?|/Home.\n"), model.CommonReadOptions{}, mpl2.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	fmt.Println(len(warnings), "warnings")
	// Output:
	// 1.2s 4.5s [Where are you? <i>Home.</i>]
	// 0 warnings
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCues(model.Cue{Start: 1240 * time.Millisecond, End: 4460 * time.Millisecond, Lines: []string{"Where are you?", "<i>Home.</i>"}})
	if err := mpl2.Write(os.Stdout, s, model.CommonWriteOptions{}, mpl2.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// [12][45]Where are you?|/Home.
}

func ExampleDetect() {
	fmt.Println(mpl2.Detect([]byte("[12][45]Where are you?\n")))
	fmt.Println(mpl2.Detect([]byte("[00:12.45]Where are you?\n")))
	// Output:
	// true
	// false
}
