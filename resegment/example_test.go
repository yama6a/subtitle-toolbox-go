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
