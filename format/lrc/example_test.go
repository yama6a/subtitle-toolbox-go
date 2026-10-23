package lrc_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/lrc"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	content := "[ti:Morning Train]\n[offset:+500]\n[00:12.00][01:15.30]The train leaves\n[00:17.20]\n" +
		"[00:21.10]<00:21.10>Bread <00:21.60>is <00:21.90>warm\n"
	s, _, err := lrc.Read(strings.NewReader(content), model.CommonReadOptions{}, lrc.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	title, _ := s.Metadata(model.MetadataTitle)
	fmt.Println(title)
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// Morning Train
	// 11.5s 16.7s The train leaves
	// 20.6s 1m14.8s <00:00:20.600>Bread <00:00:21.100>is <00:00:21.400>warm
	// 1m14.8s 1m19.8s The train leaves
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataArtist, "Station Choir")
	s.AddCues(
		model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{"<i>First</i> line"}},
		model.Cue{Start: 3 * time.Second, End: 5 * time.Second, Lines: []string{"<00:00:03.000>Slow <00:00:03.550>river"}},
	)
	err := lrc.Write(os.Stdout, s, model.CommonWriteOptions{BOM: model.BOMRemove}, lrc.WriteOptions{})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// [ar:Station Choir]
	// [00:01.00] First line
	// [00:03.00] <00:03.00>Slow <00:03.55>river
}

func ExampleDetect() {
	fmt.Println(lrc.Detect([]byte("[ti:Rain]\n[00:12.00]Rain on the roof\n")))
	fmt.Println(lrc.Detect([]byte("[10][25]Rain\n")))
	// Output:
	// true
	// false
}
