// Package httppost sends the JSON requests of the translation engines.
package httppost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/translation"
)

const (
	maxAttempts = 4
	maxWait     = 60 * time.Second
	baseLimit   = 1 << 20
)

var baseURLRegexp = regexp.MustCompile(`(?i)^https?://[^/]`)

// Wait sleeps for d or until ctx ends. Engines hold it in an unexported field, so tests replace it.
type Wait func(ctx context.Context, d time.Duration) error

// Sleep is the Wait of the engines. It returns ctx.Err() when ctx ends first.
func Sleep(ctx context.Context, d time.Duration) error {
	if ctx.Err() != nil {
		return ctx.Err() //nolint:wrapcheck // Post wraps it, and the contract of Sleep is ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err() //nolint:wrapcheck // Post wraps it, and the contract of Sleep is ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Post sends body as a JSON POST to endpoint with header, and sends it again after HTTP 429, 500, 502, 503 and 504.
// It returns the last status and body. The answer may hold up to 1 MiB plus 16 bytes for each byte of body.
func Post(ctx context.Context, client translation.HTTPClient, endpoint string, header http.Header, body []byte, wait Wait) (int, []byte, error) {
	header = header.Clone()
	if header == nil {
		header = http.Header{}
	}
	header.Set("Content-Type", "application/json")
	limit := baseLimit + 16*int64(len(body))

	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return 0, nil, &translation.ServiceError{Msg: "The request URL is not valid: " + reason(err) + "."}
		}
		req.Header = header.Clone()
		resp, err := client.Do(req)
		if err != nil {
			return 0, nil, failure(ctx, req, 0, err)
		}
		answer, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		_ = resp.Body.Close()
		if err != nil {
			return 0, nil, failure(ctx, req, resp.StatusCode, err)
		}
		if int64(len(answer)) > limit {
			return 0, nil, &translation.ServiceError{
				Status: resp.StatusCode,
				Msg:    "The answer from " + req.URL.Hostname() + " has more than " + strconv.FormatInt(limit, 10) + " bytes.",
			}
		}
		if !retryable(resp.StatusCode) || attempt == maxAttempts {
			return resp.StatusCode, answer, nil
		}
		if err := wait(ctx, retryDelay(resp, attempt)); err != nil {
			return 0, nil, fmt.Errorf("translation: %w", err)
		}
	}
}

func reason(err error) string {
	if uerr, ok := errors.AsType[*url.Error](err); ok {
		return uerr.Err.Error()
	}
	return err.Error()
}

func failure(ctx context.Context, req *http.Request, status int, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("translation: %w", ctx.Err())
	}
	return &translation.ServiceError{
		Status: status,
		Msg:    "The request to " + req.URL.Hostname() + " failed: " + reason(err) + ".",
		Err:    err,
	}
}

func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// retryDelay returns the wait before the retry that follows attempt. Retry-After wins over 1, 2 and 4 s.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if d, ok := retryAfter(resp.Header); ok {
		return d
	}
	return time.Second << (attempt - 1)
}

// retryAfter reads Retry-After as seconds or as an HTTP date, measured against the Date header of the same answer.
func retryAfter(h http.Header) (time.Duration, bool) {
	v := h.Get("Retry-After")
	if v != "" && strings.Trim(v, "0123456789") == "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n > int64(maxWait/time.Second) {
			return maxWait, true
		}
		return time.Duration(n) * time.Second, true
	}
	at, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	now, err := http.ParseTime(h.Get("Date"))
	if err != nil {
		now = time.Now()
	}
	return min(max(at.Sub(now), 0), maxWait), true
}

// JSON encodes v without HTML escapes and without the final line feed.
func JSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encode the request body: %w", err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// ValidBaseURL reports whether s matches (?i)^https?://[^/].
func ValidBaseURL(s string) bool {
	return baseURLRegexp.MatchString(s)
}

// CleanKey reports whether key has no byte from 0x00 to 0x1F, no 0x7F, and no space at the start or end.
func CleanKey(key string) bool {
	if strings.HasPrefix(key, " ") || strings.HasSuffix(key, " ") {
		return false
	}
	for i := range len(key) {
		if key[i] < 0x20 || key[i] == 0x7f {
			return false
		}
	}
	return true
}

// Redact replaces each key in msg with ***. An empty key returns msg unchanged.
func Redact(msg, key string) string {
	if key == "" {
		return msg
	}
	return strings.ReplaceAll(msg, key, "***")
}
