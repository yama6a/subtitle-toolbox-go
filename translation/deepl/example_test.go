package deepl_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/yama6a/subtitle-toolbox-go/v3/translation/deepl"
)

func ExampleNew() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"translations": [{"text": "Hallo <x1>Welt</x1>"}]}`)
	}))
	defer srv.Close()

	engine, err := deepl.New(deepl.Options{APIKey: "secret:fx", BaseURL: srv.URL})
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
	// Output:
	// Hallo <x1>Welt</x1>
}
