package openai_test

import (
	"context"
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/v3/translation/openai"
)

func ExampleNew() {
	engine, err := openai.New(openai.Options{BaseURL: "http://localhost:11434/v1", Model: "llama3"})
	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = engine.Translate(context.Background(), nil, "en", "de")
	fmt.Println(err)
	fmt.Println(openai.Options{BaseURL: "http://localhost:11434/v1", Model: "llama3", APIKey: "secret"})
	// Output:
	// <nil>
	// {BaseURL:http://localhost:11434/v1 Model:llama3 APIKey:*** Prompt:}
}
