package translation_test

import (
	"context"
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation"
)

type stationEngine struct{}

func (stationEngine) Translate(_ context.Context, texts []string, _, _ string) ([]string, error) {
	out := make([]string, len(texts))
	for i := range texts {
		out[i] = "Der Zug nach Basel faehrt um <x1>10:15</x1> von Gleis 4 ab."
	}
	return out, nil
}

func ExampleTranslate() {
	s := &model.Subtitle{}
	for i, text := range []string{"The train to Basel leaves", "from platform 4 at <b>10:15</b>."} {
		c := model.Cue{Start: time.Duration(i+1) * time.Second, End: time.Duration(i+2) * time.Second}
		c.SetText(text)
		s.AddCue(c)
	}

	report, err := translation.Translate(context.Background(), s, stationEngine{}, "en", "de", translation.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, c := range s.Cues() {
		fmt.Println(c.Lines)
	}
	language, _ := s.Metadata(model.MetadataLanguage)
	fmt.Println(language, len(report.Warnings))
	// Output:
	// [Der Zug nach Basel faehrt]
	// [um <b>10:15</b> von Gleis 4 ab.]
	// de 0
}
