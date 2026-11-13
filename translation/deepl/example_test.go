package deepl_test

import (
	"context"
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/translationtest"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation/deepl"
)

func ExampleNew() {
	client := &translationtest.Client{Answers: []translationtest.Answer{
		{Status: 200, Body: []byte(`{"translations": [{"text": "Hallo <x1>Welt</x1>"}]}`)},
	}}
	engine, err := deepl.New(deepl.Options{APIKey: "secret:fx", HTTPClient: client})
	if err != nil {
		fmt.Println(err)
		return
	}

	got, err := engine.Translate(context.Background(), []string{"Hello <x1>world</x1>"}, "en", "de")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(got[0])
	fmt.Println(client.Requests()[0].URL)
	// Output:
	// Hallo <x1>Welt</x1>
	// https://api-free.deepl.com/v2/translate
}
