package google_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation/google"
)

func ExampleNew() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"data": {"translations": [{"translatedText": "Le train part &agrave; <x1>10:15</x1>."}]}}`)
	}))
	defer server.Close()
	engine, err := google.New(google.Options{APIKey: "my-key", BaseURL: server.URL})
	if err != nil {
		fmt.Println(err)
		return
	}

	s := &model.Subtitle{}
	c := model.Cue{Start: time.Second, End: 2 * time.Second}
	c.SetText("The train leaves at <b>10:15</b>.")
	s.AddCue(c)
	if _, err := translation.Translate(context.Background(), s, engine, "en", "fr", translation.Options{}); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(s.Cues()[0].Lines)
	// Output: [Le train part à <b>10:15</b>.]
}
