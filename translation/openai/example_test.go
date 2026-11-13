package openai_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/yama6a/subtitle-toolbox-go/v3/translation/openai"
)

func ExampleNew() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		fmt.Fprint(w, `{"choices": [{"message": {"content": "[\"Hallo\", \"Der Zug fährt ab.\"]"}}]}`)
	}))
	defer srv.Close()

	engine, err := openai.New(openai.Options{BaseURL: srv.URL + "/v1", Model: "llama3"})
	if err != nil {
		fmt.Println(err)
		return
	}
	got, err := engine.Translate(context.Background(), []string{"Hello", "The train leaves."}, "en", "de")
	fmt.Println(got, err)
	fmt.Println(openai.Options{BaseURL: "http://localhost:11434/v1", Model: "llama3", APIKey: "secret"})
	// Output:
	// [Hallo Der Zug fährt ab.] <nil>
	// {BaseURL:http://localhost:11434/v1 Model:llama3 APIKey:*** Prompt: HTTPClient:<nil>}
}
