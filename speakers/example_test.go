package speakers_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/speakers"
)

func ExampleApply() {
	var s model.Subtitle
	s.AddCue(model.Cue{End: 2 * time.Second, Speaker: "Anna", Lines: []string{"Is the ferry on time today?"}})
	s.AddCue(model.Cue{Start: 2 * time.Second, End: 4 * time.Second, Lines: []string{"BEN: It left early."}})

	report, err := speakers.Apply(&s, speakers.Options{FromLabels: true, Style: speakers.StylePrefix})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("changed cues:", report.ChangedCues)
	for _, c := range s.All() {
		fmt.Println(c.Text())
	}
	// Output:
	// changed cues: 1
	// ANNA: Is the ferry on time today?
	// BEN: It left early.
}

func ExampleList() {
	var s model.Subtitle
	s.AddCue(model.Cue{End: time.Second, Speaker: "Anna", Lines: []string{"Hi."}})
	s.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"<v Ben>Hello. <v Anna>Bye."}})
	fmt.Println(speakers.List(&s))
	// Output: [{Anna 2} {Ben 1}]
}
