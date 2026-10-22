package diff_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/diff"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func cue(start, end time.Duration, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Lines: lines}
}

func editedPair() (oldSub, newSub *model.Subtitle) {
	oldSub, newSub = &model.Subtitle{}, &model.Subtitle{}
	oldSub.AddCues(
		cue(1*time.Second, 3*time.Second, "The train is late."),
		cue(4*time.Second, 6*time.Second, "The next stop is Milbrook."),
		cue(8*time.Second, 9*time.Second, "Thank you."),
	)
	newSub.AddCues(
		cue(0, 500*time.Millisecond, "Welcome."),
		cue(1*time.Second, 3*time.Second, "The train is late."),
		cue(4*time.Second, 6*time.Second, "The next stop is Millbrook."),
	)
	return oldSub, newSub
}

func ExampleCompare() {
	oldSub, newSub := editedPair()

	differences, err := diff.Compare(oldSub, newSub, diff.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, d := range differences {
		fmt.Println(d.Kind, d.OldIndex, d.NewIndex)
	}
	// Output:
	// added -1 0
	// text changed 1 2
	// removed 2 -1
}

func ExampleEqual() {
	oldSub, newSub := editedPair()

	same, err := diff.Equal(oldSub, oldSub, diff.Options{})
	fmt.Println(same, err)
	same, err = diff.Equal(oldSub, newSub, diff.Options{})
	fmt.Println(same, err)
	// Output:
	// true <nil>
	// false <nil>
}

func ExampleText() {
	var v1, v2 model.Subtitle
	v1.AddCues(
		model.Cue{Start: 1 * time.Second, End: 3 * time.Second, Lines: []string{"The train is late."}},
		model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{"The next stop is Milbrook."}},
		model.Cue{Start: 8 * time.Second, End: 9 * time.Second, Lines: []string{"Thank you."}},
	)
	v2.AddCues(
		model.Cue{Start: 0, End: 500 * time.Millisecond, Lines: []string{"Welcome."}},
		model.Cue{Start: 1 * time.Second, End: 3 * time.Second, Lines: []string{"The train is late."}},
		model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{"The next stop is Millbrook."}},
	)

	differences, err := diff.Compare(&v1, &v2, diff.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print(diff.Text(differences))
	// Output:
	// added: new cue 1
	// + 00:00:00.000 --> 00:00:00.500
	//   Welcome.
	//
	// text changed: old cue 2, new cue 3
	// - 00:00:04.000 --> 00:00:06.000
	//   The next stop is Milbrook.
	// + 00:00:04.000 --> 00:00:06.000
	//   The next stop is Millbrook.
	//
	// removed: old cue 3
	// - 00:00:08.000 --> 00:00:09.000
	//   Thank you.
}
