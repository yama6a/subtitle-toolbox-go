package chapters_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
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

func ExampleReadPodcast() {
	content := `{"chapters": [{"startTime": 0, "title": "Intro"}, {"startTime": 168.5, "title": "Hearing aids"}]}`
	s, _, err := chapters.ReadPodcast(strings.NewReader(content), model.CommonReadOptions{}, chapters.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	// Output:
	// 0s 2m48.5s [Intro]
	// 2m48.5s 2m48.5s [Hearing aids]
}

func ExampleWritePodcast() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{End: 168 * time.Second, Lines: []string{"Intro"}})
	s.AddCue(model.Cue{Start: 168 * time.Second, End: 300 * time.Second, Lines: []string{"Hearing aids"}})
	if err := chapters.WritePodcast(os.Stdout, s, model.CommonWriteOptions{}, chapters.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// {
	//     "version": "1.2.0",
	//     "chapters": [
	//         {
	//             "startTime": 0,
	//             "title": "Intro"
	//         },
	//         {
	//             "startTime": 168,
	//             "endTime": 300,
	//             "title": "Hearing aids"
	//         }
	//     ]
	// }
}

func ExampleReadFFMetadata() {
	content := ";FFMETADATA1\ntitle=Meetup\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=168000\ntitle=Intro\n"
	s, _, err := chapters.ReadFFMetadata(strings.NewReader(content), model.CommonReadOptions{}, chapters.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	title, _ := s.Metadata(model.MetadataTitle)
	fmt.Println(title)
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	// Output:
	// Meetup
	// 0s 2m48s [Intro]
}

func ExampleWriteFFMetadata() {
	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataTitle, "Meetup")
	s.AddCue(model.Cue{End: 168 * time.Second, Lines: []string{"Intro"}})
	if err := chapters.WriteFFMetadata(os.Stdout, s, model.CommonWriteOptions{}, chapters.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// ;FFMETADATA1
	// title=Meetup
	// [CHAPTER]
	// TIMEBASE=1/1000
	// START=0
	// END=168000
	// title=Intro
}

func ExampleReadOGM() {
	content := "CHAPTER01=00:00:00.000\nCHAPTER01NAME=Intro\nCHAPTER02=00:02:48.000\nCHAPTER02NAME=Hearing aids\n"
	s, _, err := chapters.ReadOGM(strings.NewReader(content), model.CommonReadOptions{}, chapters.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	// Output:
	// 0s 2m48s [Intro]
	// 2m48s 2m48s [Hearing aids]
}

func ExampleWriteOGM() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Lines: []string{"Intro"}})
	s.AddCue(model.Cue{Start: 168 * time.Second, Lines: []string{"Hearing aids"}})
	if err := chapters.WriteOGM(os.Stdout, s, model.CommonWriteOptions{}, chapters.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// CHAPTER01=00:00:00.000
	// CHAPTER01NAME=Intro
	// CHAPTER02=00:02:48.000
	// CHAPTER02NAME=Hearing aids
}
