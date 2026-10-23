package fixing_test

import (
	"errors"
	"fmt"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/fixing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleApply() {
	const content = "1\n00:00:01,000 --> 00:00:03,000\nlt's late. l'm here, l'll go.\n\n" +
		"2\n00:00:03,500 --> 00:00:06,000\n-Really ?\n-Yes.\n"
	s, _, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	report, err := fixing.Apply(s, fixing.Options{Language: "en"})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, f := range report.Fixes {
		fmt.Printf("cue %d %s: %q -> %q\n", f.CueIndex, f.Rule, f.Before, f.After)
	}
	for _, c := range s.All() {
		fmt.Println(c.Text())
	}
	// Output:
	// cue 0 ocrLowercaseL: "lt's late. l'm here, l'll go." -> "It's late. I'm here, I'll go."
	// cue 1 spaceBeforePunctuation: "-Really ?\n-Yes." -> "-Really?\n-Yes."
	// cue 1 dialogueDashes: "-Really?\n-Yes." -> "- Really?\n- Yes."
	// It's late. I'm here, I'll go.
	// - Really?
	// - Yes.
}

func ExamplePreview() {
	var s model.Subtitle
	s.AddCue(model.Cue{Lines: []string{"''Hi,'' she said.", "# Happy birthday #"}})

	report, err := fixing.Preview(&s, fixing.Options{
		Enable: []fixing.Rule{fixing.RuleDoubleApostrophes, fixing.RuleMusicNotes},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, f := range report.Fixes {
		fmt.Printf("%s: %+q\n", f.Rule, f.After)
	}
	c, _ := s.Cue(0)
	fmt.Printf("unchanged: %q\n", c.Text())
	// Output:
	// doubleApostrophes: "\"Hi,\" she said.\n# Happy birthday #"
	// musicNotes: "\"Hi,\" she said.\n\u266a Happy birthday \u266a"
	// unchanged: "''Hi,'' she said.\n# Happy birthday #"
}

func ExampleRules() {
	for _, r := range fixing.Rules() {
		fmt.Println(r, r.Default())
	}
	// Output:
	// adCues false
	// unbalancedTags true
	// emptyTags true
	// doubleApostrophes false
	// ocrPipe true
	// ocrZeroInWords true
	// ocrLowercaseL true
	// loneLowercaseI false
	// ellipsis true
	// unneededPeriods false
	// doubleSpaces true
	// spaceBeforePunctuation true
	// missingSpaceAfterPunctuation true
	// dialogueOnOneLine false
	// dialogueDashes true
	// musicNotes false
	// sentenceStartCase false
}

func ExampleOptionalRules() {
	fmt.Println(fixing.OptionalRules())
	// Output:
	// [adCues doubleApostrophes loneLowercaseI unneededPeriods dialogueOnOneLine musicNotes sentenceStartCase]
}

func ExampleRule_Default() {
	fmt.Println(fixing.RuleOCRPipe.Default(), fixing.RuleAdCues.Default())
	// Output:
	// true false
}

func ExampleOptions_Validate() {
	err := fixing.Options{Enable: []fixing.Rule{"ocrpipe"}}.Validate()
	fmt.Println(errors.Is(err, model.ErrInvalidArgument))
	fmt.Println(err)
	// Output:
	// true
	// stbox: invalid argument: fixing.Options.Enable: Unknown rule "ocrpipe".
}
