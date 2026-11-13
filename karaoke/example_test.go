package karaoke_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/karaoke"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleApply() {
	ms := time.Millisecond
	var s model.Subtitle
	s.AddCue(model.Cue{
		Start: 0, End: 1600 * ms, Lines: []string{"The beach was quiet."},
		Words: []model.Word{
			{Text: "The", Start: 0, End: 240 * ms},
			{Text: "beach", Start: 240 * ms, End: 710 * ms},
			{Text: "was", Start: 710 * ms, End: 950 * ms},
			{Text: "quiet.", Start: 950 * ms, End: 1600 * ms},
		},
	})
	report, err := karaoke.Apply(&s, karaoke.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(report.CuesBefore, "cue became", report.CuesAfter)
	for _, c := range s.All() {
		fmt.Println(c.Start, c.Lines[0])
	}
	// Output:
	// 1 cue became 4
	// 0s <u>The</u> beach was quiet.
	// 240ms The <u>beach</u> was quiet.
	// 710ms The beach <u>was</u> quiet.
	// 950ms The beach was <u>quiet.</u>
}
