package glyph_test

import (
	"context"
	"fmt"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v4/ocr/glyph"
)

func ExampleNew() {
	s, _, err := stbox.Load("../../testdata/pgs/text_1080p.sup", stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	engine, err := glyph.New(glyph.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	if _, err := ocr.Run(context.Background(), s, engine, ""); err != nil {
		fmt.Println(err)
		return
	}

	cue, _ := s.Cue(0)
	fmt.Println(cue.Lines)
	// Output:
	// [The train to Bergen leaves at 7:45.]
}
