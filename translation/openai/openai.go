// Package openai translates text with a service that offers the OpenAI chat completions API,
// such as OpenAI, Ollama, LM Studio, the llama.cpp server and vLLM.
package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation/internal/httppost"
)

// DefaultPrompt is the built-in system prompt.
const DefaultPrompt = "You translate subtitle text from {source} to {target}. The user sends a JSON array of strings. " +
	"Translate each string on its own. Keep the tags such as <x1>, </x1> and <x2/> around the words that " +
	"they mark, and keep the entities &lt;, &gt; and &amp;. Keep the line breaks. Answer only with a JSON " +
	"array of strings that has as many strings as the input, in the same order, and no other text."

const unknownSource = "the language of the text"

var thinkRegexp = regexp.MustCompile(`(?s)<think>.*?</think>`)

// Options holds the settings of New.
type Options struct {
	// BaseURL is the URL in front of /chat/completions. Required.
	BaseURL string
	// Model is the model name that the service knows. Required.
	Model string
	// APIKey is the key for Authorization: Bearer. "" sends no Authorization header.
	APIKey string
	// Prompt is the system prompt. {source} and {target} in it become the language codes. "" means DefaultPrompt.
	Prompt string
	// HTTPClient sends the requests. nil means translation.DefaultHTTPClient().
	HTTPClient translation.HTTPClient
}

// Validate checks the options. The error matches model.ErrInvalidArgument.
func (o Options) Validate() error {
	switch {
	case !httppost.ValidBaseURL(o.BaseURL):
		return invalid("BaseURL", fmt.Sprintf("The base URL must start with http:// or https://, got %q.", o.BaseURL))
	case strings.Trim(o.Model, markupcore.PHPTrimSet) == "":
		return invalid("Model", "The model must not be empty. Pass a model name such as gpt-4o-mini or llama3.")
	case o.APIKey != "" && (strings.Trim(o.APIKey, markupcore.PHPTrimSet) == "" || !httppost.CleanKey(o.APIKey)):
		return invalid("APIKey", "The API key must not be empty, have a control character, or a space at the start or end. Leave it empty for a service without a key.")
	case o.Prompt != "" && strings.Trim(o.Prompt, markupcore.PHPTrimSet) == "":
		return invalid("Prompt", "The prompt must not be empty. Leave it empty for the built-in prompt.")
	}
	return nil
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "openai.Options." + field, Msg: msg}
}

// String prints the options with the key as ***.
func (o Options) String() string {
	return fmt.Sprintf("{BaseURL:%s Model:%s APIKey:%s Prompt:%s}", o.BaseURL, o.Model, maskKey(o.APIKey), o.Prompt)
}

// GoString prints the options as Go syntax with the key as ***.
func (o Options) GoString() string {
	return fmt.Sprintf("openai.Options{BaseURL:%q, Model:%q, APIKey:%q, Prompt:%q, HTTPClient:%T}",
		o.BaseURL, o.Model, maskKey(o.APIKey), o.Prompt, o.HTTPClient)
}

func maskKey(key string) string {
	if key == "" {
		return ""
	}
	return "***"
}

// Engine is a translation.Engine for an OpenAI-compatible service.
type Engine struct {
	url     string
	options Options
	client  translation.HTTPClient
	wait    httppost.Wait
}

var _ translation.Engine = (*Engine)(nil)

// New validates opts and returns an engine.
func New(opts Options) (*Engine, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	client := opts.HTTPClient
	if client == nil {
		client = translation.DefaultHTTPClient()
	}
	return &Engine{
		url:     strings.TrimRight(opts.BaseURL, "/") + "/chat/completions",
		options: opts,
		client:  client,
		wait:    httppost.Sleep,
	}, nil
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type requestBody struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

// Translate sends all texts in one request as a JSON array. When the answer holds no usable array,
// it sends each text in a request of its own.
func (e *Engine) Translate(ctx context.Context, texts []string, source, target string) ([]string, error) {
	if len(texts) == 0 {
		return []string{}, nil
	}
	if source == "" {
		source = unknownSource
	}
	prompt := e.options.Prompt
	if prompt == "" {
		prompt = DefaultPrompt
	}
	prompt = strings.NewReplacer("{source}", source, "{target}", target).Replace(prompt)

	got, err := e.request(ctx, texts, prompt)
	if err != nil || got != nil {
		return got, err
	}
	out := make([]string, 0, len(texts))
	for _, text := range texts {
		one, err := e.request(ctx, []string{text}, prompt)
		if err != nil {
			return nil, err
		}
		if one == nil {
			return nil, e.fail(0, fmt.Sprintf("The model %q did not answer with a JSON array that holds 1 string for 1 text.", e.options.Model))
		}
		out = append(out, one[0])
	}
	return out, nil
}

// request returns nil without an error when the answer holds no array with one string per text.
func (e *Engine) request(ctx context.Context, texts []string, prompt string) ([]string, error) {
	list, err := httppost.JSON(texts)
	if err != nil {
		return nil, fmt.Errorf("translation: %w", err)
	}
	body, err := httppost.JSON(requestBody{Model: e.options.Model, Messages: []message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: string(list)},
	}})
	if err != nil {
		return nil, fmt.Errorf("translation: %w", err)
	}
	header := http.Header{}
	if e.options.APIKey != "" {
		header.Set("Authorization", "Bearer "+e.options.APIKey)
	}
	status, answer, err := httppost.Post(ctx, e.client, e.url, header, body, e.wait)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, e.statusError(status, answer)
	}
	return stringArray(answerText(answer), len(texts)), nil
}

func (e *Engine) statusError(status int, answer []byte) error {
	var data struct {
		Error any `json:"error"`
	}
	_ = json.Unmarshal(answer, &data)
	detail := ""
	switch v := data.Error.(type) {
	case string:
		detail = " " + v
	case map[string]any:
		if m, ok := v["message"].(string); ok {
			detail = " " + m
		}
	}
	switch status {
	case http.StatusUnauthorized:
		return e.fail(status, "The OpenAI-compatible service rejected the API key (HTTP 401)."+detail)
	case http.StatusTooManyRequests:
		return e.fail(status, "The OpenAI-compatible service got too many requests (HTTP 429). Wait and try again.")
	}
	return e.fail(status, "The OpenAI-compatible service answered with HTTP "+strconv.Itoa(status)+"."+detail)
}

func (e *Engine) fail(status int, msg string) error {
	return &translation.ServiceError{Status: status, Msg: httppost.Redact(msg, e.options.APIKey)}
}

// answerText returns the first of content, reasoning and reasoning_content that holds text, else "".
func answerText(answer []byte) string {
	var data struct {
		Choices []struct {
			Message map[string]any `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(answer, &data) != nil || len(data.Choices) == 0 {
		return ""
	}
	for _, field := range []string{"content", "reasoning", "reasoning_content"} {
		if s, ok := data.Choices[0].Message[field].(string); ok && strings.Trim(s, markupcore.PHPTrimSet) != "" {
			return s
		}
	}
	return ""
}

// stringArray reads the first JSON array in s, which can stand in a code block or after a think block.
// It returns nil unless the array holds count strings.
func stringArray(s string, count int) []string {
	s = thinkRegexp.ReplaceAllString(s, "")
	start, end := strings.Index(s, "["), strings.LastIndex(s, "]")
	if start < 0 || end < start {
		return nil
	}
	var values []any
	if json.Unmarshal([]byte(s[start:end+1]), &values) != nil || len(values) != count {
		return nil
	}
	out := make([]string, count)
	for i, v := range values {
		str, ok := v.(string)
		if !ok {
			return nil
		}
		out[i] = str
	}
	return out
}
