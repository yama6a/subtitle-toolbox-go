package mp4_test

import (
	"fmt"
	"os"

	"github.com/yama6a/subtitle-toolbox-go/v4/container/mp4"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleOpen() {
	file, err := os.Open("../../testdata/mp4/one_track.mp4")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		fmt.Println(err)
		return
	}
	f, err := mp4.Open(file, info.Size())
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, t := range f.Tracks() {
		fmt.Println(t.Number, t.CodecID, t.Format, t.Language)
	}
	s, _, err := f.Extract(2, model.CommonReadOptions{}, mp4.ExtractOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	c := s.Cues()[0]
	fmt.Println(c.Start, c.End, c.Lines)
	// Output:
	// 2 tx3g srt eng
	// 2s 5s [The museum opens at ten.]
}
