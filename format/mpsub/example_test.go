package mpsub_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	input := "TITLE=Harbour walk\nFORMAT=TIME\n\n1 2.5\nThe boat leaves.\n\n0.5 2\nMind the gap.\n"
	s, warnings, err := mpsub.Read(strings.NewReader(input), model.CommonReadOptions{}, mpsub.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	title, _ := s.Metadata(model.MetadataTitle)
	fmt.Println(title)
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	fmt.Println(len(warnings), "warnings")
	// Output:
	// Harbour walk
	// 1s 3.5s [The boat leaves.]
	// 4s 6s [Mind the gap.]
	// 0 warnings
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"First"}},
		model.Cue{Start: 2500 * time.Millisecond, End: 5 * time.Second, Lines: []string{"<i>Second</i>"}},
	)
	if err := mpsub.Write(os.Stdout, s, model.CommonWriteOptions{BOM: model.BOMRemove}, mpsub.WriteOptions{FrameRate: 25}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// TITLE=
	// AUTHOR=
	// TYPE=VIDEO
	// FORMAT=25
	// NOTE=Created with subtitle-toolbox-go (https://github.com/yama6a/subtitle-toolbox-go)
	//
	// 25 75
	// First
	//
	// -37 62
	// Second
}

func ExampleWriteOptions_Validate() {
	fmt.Println(mpsub.WriteOptions{FrameRate: 25}.Validate())
	fmt.Println(mpsub.WriteOptions{FrameRate: 29.97}.Validate())
	// Output:
	// <nil>
	// stbox: invalid argument: WriteOptions.MPSub.FrameRate: The MPSub frame rate must be a positive integer, got 29.97.
}

func ExampleDetect() {
	fmt.Println(mpsub.Detect([]byte("TITLE=Harbour walk\nFORMAT=TIME\n\n1 2.5\nThe boat leaves.\n")))
	fmt.Println(mpsub.Detect([]byte("TITLE=Harbour walk\n\n1 2.5\nThe boat leaves.\n")))
	// Output:
	// true
	// false
}
