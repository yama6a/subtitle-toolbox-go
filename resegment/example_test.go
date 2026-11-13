package resegment_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/resegment"
)

func ExampleSplitLong() {
	var s model.Subtitle
	s.AddCue(model.Cue{
		Start: 0, End: 11800 * time.Millisecond,
		Lines: []string{"Welcome to the city library tour. The library opens at nine and closes at six every weekday. Please keep your voices down."},
	})

	report, err := resegment.SplitLong(&s, resegment.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(report.CuesBefore, "cues became", report.CuesAfter)
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// 1 cues became 3
	// 0s 3.289s Welcome to the city library tour.
	// 3.289s 8.995s The library opens at nine and
	// closes at six every weekday.
	// 8.995s 11.8s Please keep your voices down.
}

func ExampleByWords() {
	var first, second model.Cue
	first.Start, first.End = 0, 2*time.Second
	first.Lines = []string{"One two. Three"}
	first.Words = []model.Word{
		{Text: "One", Start: 0, End: 500 * time.Millisecond},
		{Text: "two.", Start: 500 * time.Millisecond, End: time.Second},
		{Text: "Three", Start: time.Second, End: 2 * time.Second},
	}
	second.Start, second.End = 2*time.Second, 4*time.Second
	second.Lines = []string{"four five."}
	second.Words = []model.Word{
		{Text: "four", Start: 2 * time.Second, End: 3 * time.Second},
		{Text: "five.", Start: 3 * time.Second, End: 4 * time.Second},
	}
	var s model.Subtitle
	s.AddCues(first, second)

	if _, err := resegment.ByWords(&s, resegment.Options{}); err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// 0s 1s One two.
	// 1s 4s Three four five.
}
