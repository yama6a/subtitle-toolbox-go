package tmplayer_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/tmplayer"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	content := "00:00:01:Rain\n00:00:04:\n00:00:09:Sun\n"
	s, warnings, err := tmplayer.Read(strings.NewReader(content), model.CommonReadOptions{}, tmplayer.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	fmt.Println(len(warnings), "warnings")
	// Output:
	// 1s 4s [Rain]
	// 9s 14s [Sun]
	// 0 warnings
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"Where are you?", "Home."}},
		model.Cue{Start: 9 * time.Second, End: 11 * time.Second, Lines: []string{"<i>Sun</i>"}},
	)
	if err := tmplayer.Write(os.Stdout, s, model.CommonWriteOptions{}, tmplayer.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// 00:00:01:Where are you?|Home.
	// 00:00:04:
	// 00:00:09:Sun
}

func ExampleDetect() {
	fmt.Println(tmplayer.Detect([]byte("00:00:01:Rain\n")))
	fmt.Println(tmplayer.Detect([]byte("00:00:01:10,00:00:03:10,\"Rain\"\n")))
	// Output:
	// true
	// false
}
