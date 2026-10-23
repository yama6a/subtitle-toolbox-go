package dual_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/dual"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleFromPair() {
	var english, german model.Subtitle
	english.AddCue(model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"Where are you going?"}})
	german.AddCue(model.Cue{Start: 1200 * time.Millisecond, End: 3900 * time.Millisecond, Lines: []string{"Wohin gehst du?"}})

	result, err := dual.FromPair(&english, &german, dual.Options{SecondaryStyle: "i"})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range result.All() {
		fmt.Println(c.Start, c.End)
		fmt.Println(c.Text())
	}
	// Output:
	// 1s 4s
	// Where are you going?
	// <i>Wohin gehst du?</i>
}

func ExampleFromPair_topBottom() {
	var english, german model.Subtitle
	english.AddCue(model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"Where are you going?"}})
	german.AddCue(model.Cue{Start: 1250 * time.Millisecond, End: 3700 * time.Millisecond, Lines: []string{"Wohin gehst du?"}})

	result, err := dual.FromPair(&english, &german, dual.Options{Mode: dual.ModeTopBottom})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range result.All() {
		fmt.Println(c.Start, c.End, c.Alignment, c.Text())
	}
	// Output:
	// 1s 4s BottomCenter Where are you going?
	// 1s 3.7s TopCenter Wohin gehst du?
}
