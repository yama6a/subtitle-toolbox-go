package model_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleSubtitle() {
	var s model.Subtitle
	s.AddCue(model.Cue{Start: 3 * time.Second, End: 5 * time.Second, Lines: []string{"World"}})
	s.AddCue(model.Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"  <i>Hello</i>\t"}})

	for i, c := range s.All() {
		fmt.Println(i, c.Start, c.End, c.Lines)
	}
	// Output:
	// 0 1.5s 4s [<i>Hello</i>]
	// 1 3s 5s [World]
}

func ExampleSubtitle_SetCue() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: 1 * time.Second, Lines: []string{"A"}},
		model.Cue{Start: 2 * time.Second, Lines: []string{"B"}},
		model.Cue{Start: 3 * time.Second, Lines: []string{"C"}},
	)

	c, _ := s.Cue(0) // a copy of A
	c.Start = 4 * time.Second
	_ = s.SetCue(0, c) // A moves to index 2, B and C move down

	for _, c := range s.All() {
		fmt.Print(c.Text(), " ")
	}
	// Output: B C A
}

func ExampleSubtitle_AddComment() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: 1 * time.Second, Lines: []string{"first"}},
		model.Cue{Start: 2 * time.Second, Lines: []string{"second"}},
	)
	_ = s.AddComment("NOTE before the second cue", 1)

	s.AddCue(model.Cue{Start: 0, Lines: []string{"new first"}}) // the second cue moves to index 2

	fmt.Println(s.Comments())
	// Output: [{NOTE before the second cue 2}]
}

func ExampleSubtitle_CuesAt() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: 1 * time.Second, End: 4 * time.Second},
		model.Cue{Start: 3 * time.Second, End: 6 * time.Second},
		model.Cue{Start: 4 * time.Second, End: 8 * time.Second},
	)

	fmt.Println(s.CuesAt(4 * time.Second))
	i, ok := s.CueIndexAt(2 * time.Second)
	fmt.Println(i, ok)
	between, _ := s.CuesBetween(0, 3*time.Second)
	fmt.Println(between)
	// Output:
	// [1 2]
	// 0 true
	// [0]
}

func ExampleSubtitle_ReplaceCues() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: 1 * time.Second, End: 2 * time.Second, Lines: []string{"one"}},
		model.Cue{Start: 2 * time.Second, End: 4 * time.Second, Lines: []string{"two", "three"}},
	)
	_ = s.AddComment("before two", 1)

	split := []model.Cue{
		{Start: 1 * time.Second, End: 2 * time.Second, Lines: []string{"one"}},
		{Start: 2 * time.Second, End: 3 * time.Second, Lines: []string{"two"}},
		{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"three"}},
	}
	s.ReplaceCues(split, func(old int) int { return old })

	fmt.Println(s.Len(), s.Comments())
	// Output: 3 [{before two 1}]
}

func ExampleCue_SetText() {
	var c model.Cue
	c.SetText("Hello\r\n\r\n  world  ")

	fmt.Printf("%q\n", c.Lines)
	// Output: ["Hello" "world"]
}

func ExampleCue_MapWordTimestamps() {
	c := model.Cue{Lines: []string{"<00:00:01.000>Hi <00:00:01.400>there"}}
	c.MapWordTimestamps(func(d time.Duration) time.Duration { return d + 2*time.Second })

	fmt.Println(c.Text())
	// Output: <00:00:03.000>Hi <00:00:03.400>there
}

func ExampleAlignmentFromNumpad() {
	a, ok := model.AlignmentFromNumpad(8)
	fmt.Println(a, ok, a.Row(), a.Column())
	// Output: TopCenter true 2 2
}
