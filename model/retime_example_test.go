package model_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleSubtitle_ConvertFrameRate() {
	var s model.Subtitle
	s.AddCue(model.Cue{Start: 100 * time.Second, End: 102500 * time.Millisecond, Lines: []string{"Hello"}})

	if err := s.ConvertFrameRate(25, 23.976); err != nil {
		fmt.Println(err)
		return
	}

	c, _ := s.Cue(0)
	fmt.Println(c.Start, c.End)
	// Output: 1m44.271s 1m46.878s
}

func ExampleSubtitle_SyncByPoints() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: 10 * time.Second, End: 12 * time.Second, Lines: []string{"first"}},
		model.Cue{Start: 300 * time.Second, End: 302 * time.Second, Lines: []string{"middle"}},
		model.Cue{Start: 1200 * time.Second, End: 1202 * time.Second, Lines: []string{"last"}},
	)

	err := s.SyncByPoints([]model.SyncPoint{
		{Old: 10 * time.Second, New: 12 * time.Second},
		{Old: 600 * time.Second, New: 610 * time.Second},
		{Old: 1200 * time.Second, New: 1205 * time.Second},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// 12s 14.027s first
	// 5m5.932s 5m7.959s middle
	// 20m5s 20m6.983s last
}

func ExampleSubtitle_ShiftRange() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: 10 * time.Second, End: 12 * time.Second, Lines: []string{"intro"}},
		model.Cue{Start: 20 * time.Second, End: 22 * time.Second, Lines: []string{"scene"}},
	)

	if err := s.ShiftRange(1500*time.Millisecond, new(15*time.Second), nil); err != nil {
		fmt.Println(err)
		return
	}

	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// 10s 12s intro
	// 21.5s 23.5s scene
}
