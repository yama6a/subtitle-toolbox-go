package timing_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/model"
	"github.com/yama6a/subtitle-toolbox-go/v2/timing"
)

func ExampleApply() {
	at := func(start, end float64) model.Cue {
		return model.Cue{
			Start: time.Duration(start * float64(time.Second)), End: time.Duration(end * float64(time.Second)),
			Lines: []string{"Text"},
		}
	}
	var s model.Subtitle
	s.AddCues(at(8, 10), at(10.292, 12), at(62.708, 65), at(67, 69.75))
	shots := []time.Duration{62500 * time.Millisecond, 70 * time.Second}

	report, err := timing.Apply(&s, timing.Options{FrameRate: model.FrameRate{Num: 24, Den: 1}, ShotChanges: shots})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("moved starts:", report.MovedStarts, "moved ends:", report.MovedEnds)
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End)
	}
	// Output:
	// moved starts: 1 moved ends: 2
	// 8s 10.208s
	// 10.292s 12s
	// 1m2.5s 1m5s
	// 1m7s 1m9.917s
}
