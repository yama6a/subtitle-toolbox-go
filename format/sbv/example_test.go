package sbv_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/sbv"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	content := "0:00:01.500,0:00:04.000\nWhere are you going?\n\n0:00:04.500,0:00:06.000\n<i>Home.</i>\n"
	s, warnings, err := sbv.Read(strings.NewReader(content), model.CommonReadOptions{}, sbv.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	fmt.Println(len(warnings), "warnings")
	// Output:
	// 1.5s 4s [Where are you going?]
	// 4.5s 6s [&lt;i&gt;Home.&lt;/i&gt;]
	// 0 warnings
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"<i>Fish &amp; Chips</i>"}},
		model.Cue{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"On the menu"}},
	)
	err := sbv.Write(os.Stdout, s, model.CommonWriteOptions{}, sbv.WriteOptions{})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// 0:00:01.500,0:00:04.000
	// Fish & Chips
	//
	// 0:00:05.000,0:00:06.000
	// On the menu
}

func ExampleDetect() {
	fmt.Println(sbv.Detect([]byte("0:00:01.500,0:00:04.000\nHello\n")))
	fmt.Println(sbv.Detect([]byte("1\n00:00:01,500 --> 00:00:04,000\nHello\n")))
	// Output:
	// true
	// false
}
