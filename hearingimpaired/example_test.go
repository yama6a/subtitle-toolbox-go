package hearingimpaired_test

import (
	"errors"
	"fmt"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/hearingimpaired"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleApply() {
	const content = "1\n00:00:01,000 --> 00:00:03,000\n[DOOR SLAMS]\n\n" +
		"2\n00:00:03,500 --> 00:00:06,000\n- [gasps] No!\n- MARY: Yes.\n\n" +
		"3\n00:00:06,500 --> 00:00:09,000\n<i>(whispering) Over here.</i>\n"
	s, _, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	report, err := hearingimpaired.Apply(s, hearingimpaired.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("removed lines:", report.RemovedLines, "removed cues:", report.RemovedCues)
	for _, c := range s.All() {
		fmt.Println(c.Start, c.Text())
	}
	// Output:
	// removed lines: 1 removed cues: 1
	// 3.5s - No!
	// - Yes.
	// 6.5s <i>Over here.</i>
}

func ExampleIsAnnotation() {
	for _, line := range []string{"[DOOR SLAMS]", "JOHN: Hi.", "Note: this stays.", "# Never gonna stop #"} {
		annotation, err := hearingimpaired.IsAnnotation(line, hearingimpaired.Options{})
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(line, annotation)
	}
	// Output:
	// [DOOR SLAMS] true
	// JOHN: Hi. true
	// Note: this stays. false
	// # Never gonna stop # false
}

func ExampleOptions_Validate() {
	opts := hearingimpaired.Options{CustomBrackets: []hearingimpaired.BracketPair{{Open: "{"}}}
	err := opts.Validate()
	fmt.Println(errors.Is(err, model.ErrInvalidArgument))
	fmt.Println(err)
	// Output:
	// true
	// stbox: invalid argument: hearingimpaired.Options.CustomBrackets: Each custom bracket pair must hold two non-empty strings, for example "{" and "}".
}
