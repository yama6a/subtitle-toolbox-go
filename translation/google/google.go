// Package google translates text with Google Cloud Translation Basic (v2).
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation/internal/httppost"
)

// MaxTextsPerRequest is the most texts that the engine sends in one request.
const MaxTextsPerRequest = 128

const (
	defaultBaseURL = "https://translation.googleapis.com"
	path           = "/language/translate/v2"
)

// Options holds the settings of New.
type Options struct {
	// APIKey is an API key of a Google Cloud project. It is required.
	APIKey string
	// BaseURL is the scheme and host of the service. "" means https://translation.googleapis.com.
	BaseURL string
	// HTTPClient sends the requests. nil means translation.DefaultHTTPClient().
	HTTPClient translation.HTTPClient
}

// Validate checks the key and the base URL. The error matches model.ErrInvalidArgument.
func (o Options) Validate() error {
	switch {
	case strings.Trim(o.APIKey, markupcore.PHPTrimSet) == "":
		return &model.InvalidArgumentError{
			Option: "google.Options.APIKey",
			Msg:    "The API key must not be empty. Pass the key of your Google Cloud project.",
		}
	case !httppost.CleanKey(o.APIKey):
		return &model.InvalidArgumentError{
			Option: "google.Options.APIKey",
			Msg:    "The API key must not have a control character, or a space at the start or end.",
		}
	case o.BaseURL != "" && !httppost.ValidBaseURL(o.BaseURL):
		return &model.InvalidArgumentError{
			Option: "google.Options.BaseURL",
			Msg:    fmt.Sprintf("The base URL must start with http:// or https://, got %q.", o.BaseURL),
		}
	}
	return nil
}

// String prints the options with the key as ***.
func (o Options) String() string {
	return fmt.Sprintf("{APIKey:*** BaseURL:%q HTTPClient:%T}", o.BaseURL, o.HTTPClient)
}

// GoString prints the options as Go syntax with the key as ***.
func (o Options) GoString() string {
	return fmt.Sprintf("google.Options{APIKey:\"***\", BaseURL:%q, HTTPClient:%T}", o.BaseURL, o.HTTPClient)
}

// Engine is a translation.Engine for Google Cloud Translation.
type Engine struct {
	endpoint string
	key      string
	client   translation.HTTPClient
	wait     httppost.Wait
}

var _ translation.Engine = (*Engine)(nil)

// New checks opts and returns an engine.
func New(opts Options) (*Engine, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	base := opts.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	client := opts.HTTPClient
	if client == nil {
		client = translation.DefaultHTTPClient()
	}
	return &Engine{endpoint: strings.TrimRight(base, "/") + path, key: opts.APIKey, client: client, wait: httppost.Sleep}, nil
}

// Translate translates the texts with format "html" and returns the answers as received.
// An empty source lets Google detect the language.
func (e *Engine) Translate(ctx context.Context, texts []string, source, target string) ([]string, error) {
	out := make([]string, 0, len(texts))
	for chunk := range slices.Chunk(texts, MaxTextsPerRequest) {
		got, err := e.request(ctx, chunk, source, target)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

type requestBody struct {
	Q      []string `json:"q"`
	Target string   `json:"target"`
	Format string   `json:"format"`
	Source string   `json:"source,omitempty"`
}

type errorAnswer struct {
	Error struct {
		Message any `json:"message"`
	} `json:"error"`
}

type okAnswer struct {
	Data struct {
		Translations []any `json:"translations"`
	} `json:"data"`
}

func (e *Engine) request(ctx context.Context, texts []string, source, target string) ([]string, error) {
	body, err := httppost.JSON(requestBody{Q: texts, Target: target, Format: "html", Source: source})
	if err != nil {
		return nil, err
	}
	header := http.Header{"X-Goog-Api-Key": {e.key}}
	status, answer, err := httppost.Post(ctx, e.client, e.endpoint, header, body, e.wait)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, e.statusError(status, answer)
	}

	var data okAnswer
	if json.Unmarshal(answer, &data) != nil {
		data.Data.Translations = nil
	}
	got := make([]string, 0, len(texts))
	for _, item := range data.Data.Translations {
		m, _ := item.(map[string]any)
		s, ok := m["translatedText"].(string)
		if !ok {
			break
		}
		got = append(got, s)
	}
	if len(got) != len(texts) || len(data.Data.Translations) != len(texts) {
		return nil, &translation.ServiceError{Status: status, Msg: fmt.Sprintf(
			"Google Cloud Translation returned %d translations for %d texts, or an answer that is not the JSON of translate v2.",
			len(data.Data.Translations), len(texts))}
	}
	return got, nil
}

func (e *Engine) statusError(status int, answer []byte) error {
	var detail string
	var data errorAnswer
	if json.Unmarshal(answer, &data) == nil {
		if m, ok := data.Error.Message.(string); ok {
			detail = " " + m
		}
	}
	var msg string
	switch status {
	case http.StatusForbidden:
		msg = "Google Cloud Translation refused the request (HTTP 403). Check the key and that the API is enabled." + detail
	case http.StatusTooManyRequests:
		msg = "Google Cloud Translation got too many requests (HTTP 429). Wait and try again."
	default:
		msg = "Google Cloud Translation answered with HTTP " + strconv.Itoa(status) + "." + detail
	}
	return &translation.ServiceError{Status: status, Msg: httppost.Redact(msg, e.key)}
}
