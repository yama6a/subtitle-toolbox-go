// Package deepl translates text with the DeepL API v2.
package deepl

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

// MaxTextsPerRequest is the most texts that DeepL takes in one request.
const MaxTextsPerRequest = 50

const (
	freeHost = "https://api-free.deepl.com"
	paidHost = "https://api.deepl.com"
	path     = "/v2/translate"
)

// Options holds the settings of New.
type Options struct {
	// APIKey is the DeepL API key. Required. A key that ends in ":fx" belongs to the free plan.
	APIKey string
	// BaseURL is the scheme and host of the service, for example a proxy.
	// "" picks https://api-free.deepl.com for a key that ends in ":fx", else https://api.deepl.com.
	BaseURL string
	// HTTPClient sends the requests. nil means translation.DefaultHTTPClient().
	HTTPClient translation.HTTPClient
}

// Validate checks the key and the base URL. The error matches model.ErrInvalidArgument.
func (o Options) Validate() error {
	switch {
	case strings.Trim(o.APIKey, markupcore.PHPTrimSet) == "":
		return &model.InvalidArgumentError{Option: "deepl.Options.APIKey", Msg: "The API key must not be empty. Pass the key of your DeepL account."}
	case !httppost.CleanKey(o.APIKey):
		return &model.InvalidArgumentError{Option: "deepl.Options.APIKey", Msg: "The API key must not have a control character, or a space at the start or end."}
	case o.BaseURL != "" && !httppost.ValidBaseURL(o.BaseURL):
		return &model.InvalidArgumentError{Option: "deepl.Options.BaseURL", Msg: fmt.Sprintf("The base URL must start with http:// or https://, got %q.", o.BaseURL)}
	}
	return nil
}

// String prints the options with the key as ***.
func (o Options) String() string {
	return fmt.Sprintf("{APIKey:*** BaseURL:%q HTTPClient:%T}", o.BaseURL, o.HTTPClient)
}

// GoString prints the options with the key as ***.
func (o Options) GoString() string {
	return fmt.Sprintf("deepl.Options{APIKey:\"***\", BaseURL:%q, HTTPClient:%T}", o.BaseURL, o.HTTPClient)
}

// Engine is a translation.Engine for DeepL.
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
		base = paidHost
		if strings.HasSuffix(opts.APIKey, ":fx") {
			base = freeHost
		}
	}
	client := opts.HTTPClient
	if client == nil {
		client = translation.DefaultHTTPClient()
	}
	return &Engine{endpoint: strings.TrimRight(base, "/") + path, key: opts.APIKey, client: client, wait: httppost.Sleep}, nil
}

// request is the JSON body. DeepL takes the keys in any order, PHP sends them in this one.
//
//nolint:tagliatelle // the DeepL API names the keys in snake case
type request struct {
	Text           []string `json:"text"`
	TargetLang     string   `json:"target_lang"`
	TagHandling    string   `json:"tag_handling"`
	SplitSentences string   `json:"split_sentences"`
	SourceLang     string   `json:"source_lang,omitempty"`
}

// Translate translates the texts with tag_handling "xml". An empty source lets DeepL detect the language.
// A region in source, such as "-US" in "en-US", does not go to DeepL.
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

func (e *Engine) request(ctx context.Context, texts []string, source, target string) ([]string, error) {
	// nonewlines keeps the 2 lines of a cue in one sentence.
	req := request{Text: texts, TargetLang: asciiUpper(target), TagHandling: "xml", SplitSentences: "nonewlines"}
	if source != "" {
		before, _, _ := strings.Cut(source, "-")
		req.SourceLang = asciiUpper(before)
	}
	body, err := httppost.JSON(req)
	if err != nil {
		return nil, err
	}
	header := http.Header{}
	header.Set("Authorization", "DeepL-Auth-Key "+e.key)
	status, answer, err := httppost.Post(ctx, e.client, e.endpoint, header, body, e.wait)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, e.statusError(status, answer)
	}
	got, count := parseTranslations(answer)
	if got == nil || len(got) != len(texts) {
		return nil, e.serviceError(status, fmt.Sprintf("DeepL returned %d translations for %d texts, or an answer that is not the JSON of /v2/translate.", count, len(texts)))
	}
	return got, nil
}

func (e *Engine) serviceError(status int, msg string) error {
	return &translation.ServiceError{Status: status, Msg: httppost.Redact(msg, e.key)}
}

func (e *Engine) statusError(status int, answer []byte) error {
	switch status {
	case http.StatusForbidden:
		return e.serviceError(status, "DeepL rejected the API key (HTTP 403). Check the key and its plan.")
	case http.StatusTooManyRequests:
		return e.serviceError(status, "DeepL got too many requests (HTTP 429). Wait and try again.")
	case 456:
		return e.serviceError(status, "The DeepL character quota is used up (HTTP 456).")
	}
	msg := "DeepL answered with HTTP " + strconv.Itoa(status) + "."
	var body struct {
		Message *string `json:"message"`
	}
	if json.Unmarshal(answer, &body) == nil && body.Message != nil {
		msg += " " + *body.Message
	}
	return e.serviceError(status, msg)
}

// parseTranslations returns the texts of translations[] and the length of that list. The list is 0 when the answer
// has no translations list. The texts are nil when an entry has no string text.
func parseTranslations(answer []byte) (texts []string, count int) {
	var data struct {
		Translations []json.RawMessage `json:"translations"`
	}
	if json.Unmarshal(answer, &data) != nil {
		return nil, 0
	}
	texts = make([]string, 0, len(data.Translations))
	for _, raw := range data.Translations {
		var entry struct {
			Text *string `json:"text"`
		}
		if json.Unmarshal(raw, &entry) != nil || entry.Text == nil {
			return nil, len(data.Translations)
		}
		texts = append(texts, *entry.Text)
	}
	return texts, len(texts)
}

func asciiUpper(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - 'a' + 'A'
		}
		return r
	}, s)
}
