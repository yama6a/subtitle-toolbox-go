package chapters_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func ExampleReadYouTube() {
	content := "Chapters:\n0:00 Intro\n(2:48) Hearing aids\nThe big players: 1:31:50\n"
	s, _, err := chapters.ReadYouTube(strings.NewReader(content), model.CommonReadOptions{}, chapters.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	// Output:
	// 0s 2m48s [Intro]
	// 2m48s 1h31m50s [Hearing aids]
	// 1h31m50s 1h31m50s [The big players]
}

func ExampleWriteYouTube() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Lines: []string{"Intro"}})
	s.AddCue(model.Cue{Start: 3600e9, Lines: []string{"Main part"}})
	if err := chapters.WriteYouTube(os.Stdout, s, model.CommonWriteOptions{}, chapters.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// 0:00 Intro
	// 1:00:00 Main part
}
