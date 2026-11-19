package ocr_test

import (
	"context"
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/edit/ocr"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/ocr/ocrtest"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func ExampleApply() {
	s := &model.Subtitle{}
	if err := s.AddCue(model.Cue{
		Start: time.Second,
		End:   3 * time.Second,
		Image: &model.Image{PNG: []byte("PNG"), X: 640, Y: 940, Width: 1, Height: 1, ScreenWidth: 1920, ScreenHeight: 1080},
	}); err != nil {
		panic(err)
	}
	engine := &ocrtest.Engine{Text: ocr.Text{Lines: []string{"Next stop: Bergen"}}}

	report, err := ocr.Apply(context.Background(), s, engine, ocr.English)
	if err != nil {
		fmt.Println(err)
		return
	}

	cue, _ := s.Cue(0)
	fmt.Println(cue.Lines, cue.Image != nil, len(report.Texts))
	fmt.Println(engine.Calls[0].Language)
	// Output:
	// [Next stop: Bergen] true 1
	// eng
}
