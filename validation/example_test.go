package validation_test

import (
	"fmt"
	"strings"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/validation"
)

func ExampleCheck() {
	const content = "1\n00:00:01,000 --> 00:00:03,000\n<i>The train leaves at noon.</i>\n\n" +
		"2\n00:00:02,500 --> 00:00:05,000\n[BELL RINGS]\nANNA: We need two tickets for the long ride to the coast.\n"
	r := strings.NewReader(content)
	s, _, err := stbox.Parse(r, r.Size(), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	violations, err := validation.Check(s, validation.Rules{MaxCharactersPerLine: 42, NoOverlap: true})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, v := range violations {
		fmt.Println("cue", v.Cue+1, v.Rule, v.Value)
	}
	// Output:
	// cue 2 maxCharactersPerLine 57
	// cue 2 noOverlap 0.5
}

func ExampleNetflixEnglish() {
	const content = "1\n00:00:01,000 --> 00:00:03,000\n<i>The train leaves at noon.</i>\n\n" +
		"2\n00:00:02,500 --> 00:00:02,900\nANNA: We need two tickets for the long ride to the coast.\n"
	r := strings.NewReader(content)
	s, _, err := stbox.Parse(r, r.Size(), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	rules, err := validation.NetflixEnglish(stbox.FrameRate23976)
	if err != nil {
		fmt.Println(err)
		return
	}
	violations, err := validation.Check(s, rules)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, v := range violations {
		fmt.Println("cue", v.Cue+1, v.Rule, v.Value)
	}
	// Output:
	// cue 2 maxCharactersPerLine 57
	// cue 2 maxCharactersPerSecond 142.5
	// cue 2 minDuration 0.4
	// cue 2 noOverlap 0.5
}

func ExampleCheckYouTubeChapters() {
	s := &stbox.Subtitle{}
	s.AddCue(stbox.Cue{Start: 5 * time.Second, End: 9500 * time.Millisecond})
	s.AddCue(stbox.Cue{Start: 9500 * time.Millisecond, End: 30 * time.Second})
	for _, v := range validation.CheckYouTubeChapters(s) {
		fmt.Println("cue", v.Cue, v.Rule, v.Value)
	}
	// Output:
	// cue 0 firstChapterAtZero 5
	// cue -1 minChapters 2
	// cue 0 minDuration 4.5
}
