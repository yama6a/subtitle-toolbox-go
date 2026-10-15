package model_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleSubtitle_Append() {
	var part1 model.Subtitle
	part1.AddCue(model.Cue{Start: 4200 * time.Millisecond, End: 7450 * time.Millisecond, Lines: []string{"The ferry leaves at seven."}})
	_ = part1.AddComment("End of part one", 1)

	var part2 model.Subtitle
	part2.AddCue(model.Cue{Start: 1250 * time.Millisecond, End: 4 * time.Second, Lines: []string{"PART TWO"}})

	if err := part1.Append(&part2, 3130*time.Second); err != nil {
		fmt.Println(err)
		return
	}

	for _, c := range part1.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	fmt.Println(part1.Comments())
	// Output:
	// 4.2s 7.45s The ferry leaves at seven.
	// 52m11.25s 52m14s PART TWO
	// [{End of part one 1}]
}
