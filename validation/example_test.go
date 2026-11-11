package validation_test

import (
	"fmt"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go/v2"
	"github.com/yama6a/subtitle-toolbox-go/v2/validation"
)

func ExampleCheck() {
	const content = "1\n00:00:01,000 --> 00:00:03,000\n<i>The train leaves at noon.</i>\n\n" +
		"2\n00:00:02,500 --> 00:00:05,000\n[BELL RINGS]\nANNA: We need two tickets for the long ride to the coast.\n"
	s, _, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{})
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
