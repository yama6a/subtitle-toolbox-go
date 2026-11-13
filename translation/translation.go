// Package translation translates the cue text of a subtitle with a machine translation engine.
package translation

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// Engine translates texts. Implement it to add a translation service.
type Engine interface {
	// Translate returns one translation per text, in the same order. The texts hold placeholders such as
	// <x1>...</x1> and <x2/> and the entities &lt;, &gt; and &amp;, which the translations must keep.
	// An empty source lets the service detect the language.
	Translate(ctx context.Context, texts []string, source, target string) ([]string, error)
}

// HTTPClient sends an HTTP request. *http.Client implements it.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Options holds the settings of Translate. The zero value joins sentences of up to 3 cues
// and sends up to 5000 characters per engine call.
type Options struct {
	// NoJoinSentences sends each cue as its own text.
	NoJoinSentences bool
	// MaxCuesPerSentence is the most cues in one text. 0 means DefaultMaxCuesPerSentence.
	MaxCuesPerSentence int
	// MaxCharactersPerRequest is the most grapheme clusters of the encoded texts in one engine call.
	// A longer text goes alone. 0 means DefaultMaxCharactersPerRequest.
	MaxCharactersPerRequest int
}

// The defaults of Options.
const (
	DefaultMaxCuesPerSentence      = 3
	DefaultMaxCharactersPerRequest = 5000
)

// Validate checks the limits. The error matches model.ErrInvalidArgument.
func (o Options) Validate() error {
	if o.MaxCuesPerSentence < 0 {
		return &model.InvalidArgumentError{
			Option: "translation.Options.MaxCuesPerSentence",
			Msg:    "The maximum cues per sentence must be at least 1, got " + strconv.Itoa(o.MaxCuesPerSentence) + ".",
		}
	}
	if o.MaxCharactersPerRequest < 0 {
		return &model.InvalidArgumentError{
			Option: "translation.Options.MaxCharactersPerRequest",
			Msg:    "The maximum characters per request must be at least 1, got " + strconv.Itoa(o.MaxCharactersPerRequest) + ".",
		}
	}
	return nil
}

// Warning is a problem with the translation of one cue.
type Warning struct {
	Cue     int // 0-based cue index
	Message string
}

// Report holds the warnings of Translate, sorted by cue index. Nil means no warning.
type Report struct {
	Warnings []Warning
}

// ErrService matches every error of a translation service.
var ErrService = errors.New("translation: the translation service failed")

// ServiceError is an error of a translation service.
type ServiceError struct {
	Status int    // the HTTP status, 0 when no response arrived
	Msg    string // the message, without the key
	Err    error  // the cause, for example a *url.Error, or nil
}

// Error returns "translation: " and the message.
func (e *ServiceError) Error() string { return "translation: " + e.Msg }

// Unwrap returns the cause.
func (e *ServiceError) Unwrap() error { return e.Err }

// Is reports whether target is ErrService.
func (e *ServiceError) Is(target error) bool { return target == ErrService }

var _ error = (*ServiceError)(nil)

// DefaultHTTPClient returns a new client with a connect timeout of 10 s and a total timeout of 120 s.
// It follows no redirect, so a 3xx answer reaches the engine as it is and no key goes to another host.
func DefaultHTTPClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // the standard library sets a *http.Transport
	t.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	return &http.Client{
		Transport:     t,
		Timeout:       120 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
