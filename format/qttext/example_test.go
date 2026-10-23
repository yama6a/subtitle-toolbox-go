package qttext_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/qttext"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	content := "{QTtext} {timeScale:30}\n[00:00:01.15]\nThe museum opens at ten.\n[00:00:04.00]\n\n" +
		"[00:00:05.00]\n{italic}Closing time.\n"
	s, _, err := qttext.Read(strings.NewReader(content), model.CommonReadOptions{}, qttext.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// 1.5s 4s The museum opens at ten.
	// 5s 10s <i>Closing time.</i>
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"<i>Sung</i> and <b>loud</b>"}},
		model.Cue{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"<i>Sung</i>"}},
	)
	err := qttext.Write(os.Stdout, s, model.CommonWriteOptions{}, qttext.WriteOptions{})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// {QTtext} {font:Tahoma}
	// {plain} {size:20}
	// {timeScale:30}
	// {width:160} {height:32}
	// {timestamps:absolute} {language:0}
	// [00:00:01.00]
	// {italic}Sung{plain} and {bold}loud
	// [00:00:02.00]
	//
	// [00:00:03.00]
	// {plain}{italic}Sung
	// [00:00:04.00]
}

func ExampleDetect() {
	fmt.Println(qttext.Detect([]byte("{QTtext} {font:Tahoma}\n[00:00:01.00]\nRain\n")))
	fmt.Println(qttext.Detect([]byte("[00:00:01.00]\nRain\n")))
	// Output:
	// true
	// false
}
