package vobsub_test

import (
	"bytes"
	"fmt"
	"os"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleRead() {
	idx, err := os.ReadFile("../../testdata/vobsub/two-tracks-pal.idx")
	if err != nil {
		fmt.Println(err)
		return
	}
	sub, err := os.ReadFile("../../testdata/vobsub/two-tracks-pal.sub")
	if err != nil {
		fmt.Println(err)
		return
	}
	s, _, err := vobsub.Read(bytes.NewReader(sub), model.CommonReadOptions{}, vobsub.ReadOptions{IDX: string(idx)})
	if err != nil {
		fmt.Println(err)
		return
	}
	language, _ := s.Metadata(model.MetadataLanguage)
	fmt.Println(language, s.Len())
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Image.Width, c.Image.Height, c.Forced)
	}
	// Output:
	// en 5
	// 1.5s 4.003s 300 40 false
	// 5.762s 8.254s 200 24 true
	// 9s 11s 120 30 false
	// 11s 13.002s 720 32 false
	// 20s 25s 64 16 false
}

func ExampleReadOptions_Validate() {
	index := -1
	fmt.Println(vobsub.ReadOptions{Index: &index}.Validate())
	// Output:
	// stbox: invalid argument: ReadOptions.VobSub.Index: The track index must be 0 or more, got -1.
}
