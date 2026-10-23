package microdvd_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	content := "{25}{75}Where are you?|{y:i}Home.\n{100}{150}{c:$0000FF}Stop!\n"
	s, _, err := microdvd.Read(strings.NewReader(content), model.CommonReadOptions{}, microdvd.ReadOptions{FrameRate: 25})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	// Output:
	// 1s 3s [Where are you? <i>Home.</i>]
	// 4s 6s [<font color="#ff0000">Stop!</font>]
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{"<i>Where are you?", "Home.</i>"}},
		model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{"Fish &amp; Chips"}},
	)
	err := microdvd.Write(os.Stdout, s, model.CommonWriteOptions{}, microdvd.WriteOptions{FrameRate: 23.976, WriteFrameRateLine: true})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// {1}{1}23.976
	// {24}{72}{y:i}Where are you?|{y:i}Home.
	// {96}{144}Fish & Chips
}

func ExampleDetect() {
	fmt.Println(microdvd.Detect([]byte("{24}{72}Hello")))
	fmt.Println(microdvd.Detect([]byte("1\n00:00:01,000 --> 00:00:02,000\nHello")))
	// Output:
	// true
	// false
}

func ExampleReadOptions_Validate() {
	fmt.Println(microdvd.ReadOptions{FrameRate: -1}.Validate())
	// Output:
	// stbox: invalid argument: ReadOptions.MicroDVD.FrameRate: The frame rate must be greater than 0, got -1.
}

func ExampleWriteOptions_Validate() {
	fmt.Println(microdvd.WriteOptions{FrameRate: 25}.Validate())
	// Output:
	// <nil>
}
