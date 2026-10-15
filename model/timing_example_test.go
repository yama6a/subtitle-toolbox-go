package model_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleSubtitle_AddLeadInOut() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"Is the ferry running today?"}},
		model.Cue{Start: 2100 * time.Millisecond, End: 3 * time.Second, Lines: []string{"Only the morning one."}},
	)

	if err := s.AddLeadInOut(200*time.Millisecond, 300*time.Millisecond, 40*time.Millisecond); err != nil {
		fmt.Println(err)
		return
	}

	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// 800ms 2.06s Is the ferry running today?
	// 2.1s 3.3s Only the morning one.
}

func ExampleSubtitle_FixOverlaps() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: time.Second, End: 5200 * time.Millisecond, Lines: []string{"Where is the harbour?"}},
		model.Cue{Start: 5100 * time.Millisecond, End: 7 * time.Second, Lines: []string{"Down the hill."}},
	)

	if err := s.FixOverlaps(time.Second * 2 / 24); err != nil {
		fmt.Println(err)
		return
	}

	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// 1s 5.017s Where is the harbour?
	// 5.1s 7s Down the hill.
}

func ExampleSubtitle_ExtendShortCues() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: time.Second, End: 1300 * time.Millisecond, Lines: []string{"Yes."}},
		model.Cue{Start: 1500 * time.Millisecond, End: 3 * time.Second, Lines: []string{"Then we go."}},
	)

	if err := s.ExtendShortCues(time.Second, 100*time.Millisecond); err != nil {
		fmt.Println(err)
		return
	}

	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// 1s 1.4s Yes.
	// 1.5s 3s Then we go.
}

func ExampleSubtitle_LimitLongCues() {
	var s model.Subtitle
	s.AddCue(model.Cue{Start: time.Second, End: 26 * time.Second, Lines: []string{"Good morning."}})

	if err := s.LimitLongCues(model.DefaultMaxDuration); err != nil {
		fmt.Println(err)
		return
	}

	c, _ := s.Cue(0)
	fmt.Println(c.Start, c.End)
	// Output: 1s 8s
}

func ExampleSubtitle_MergeShortCues() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: 62100 * time.Millisecond, End: 62600 * time.Millisecond, Lines: []string{"Wait."}},
		model.Cue{Start: 62640 * time.Millisecond, End: 63300 * time.Millisecond, Lines: []string{"Where are you"}},
		model.Cue{Start: 63320 * time.Millisecond, End: 64100 * time.Millisecond, Lines: []string{"going?"}},
	)

	if err := s.MergeShortCues(model.MergeShortCuesOptions{}); err != nil {
		fmt.Println(err)
		return
	}

	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// 1m2.1s 1m4.1s Wait. Where are you going?
}
