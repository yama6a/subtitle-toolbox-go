package httppost_test

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/translationtest"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation/internal/httppost"
)

const endpoint = "http://translate.test/v2/translate"

type waits []time.Duration

func (w *waits) wait(_ context.Context, d time.Duration) error {
	*w = append(*w, d)
	return nil
}

func status(codes ...int) []translationtest.Answer {
	out := make([]translationtest.Answer, len(codes))
	for i, c := range codes {
		out[i] = translationtest.Answer{Status: c, Body: []byte("body " + strconv.Itoa(i))}
	}
	return out
}

func retryAfter(value string, headers ...string) []translationtest.Answer {
	h := http.Header{"Retry-After": {value}}
	for i := 0; i+1 < len(headers); i += 2 {
		h.Set(headers[i], headers[i+1])
	}
	return []translationtest.Answer{{Status: 429, Header: h}, {Status: 200}}
}

func TestPost(t *testing.T) {
	t.Parallel()

	s := time.Second
	const date = "Mon, 02 Jan 2006 15:04:05 GMT"
	limit := 1<<20 + 16*len("body")
	tests := []struct {
		name         string
		answers      []translationtest.Answer
		wantStatus   int
		wantRequests int
		wantWaits    []time.Duration
		wantBody     string
	}{
		{name: "429, 503, 200", answers: status(429, 503, 200), wantStatus: 200, wantRequests: 3, wantWaits: []time.Duration{s, 2 * s}, wantBody: "body 2"},
		{name: "4 retryable answers", answers: status(429, 500, 502, 504), wantStatus: 504, wantRequests: 4, wantWaits: []time.Duration{s, 2 * s, 4 * s}, wantBody: "body 3"},
		{name: "400 is no retry", answers: status(400), wantStatus: 400, wantRequests: 1, wantBody: "body 0"},
		{name: "403 is no retry", answers: status(403), wantStatus: 403, wantRequests: 1, wantBody: "body 0"},
		{name: "Retry-After seconds", answers: retryAfter("3"), wantStatus: 200, wantRequests: 2, wantWaits: []time.Duration{3 * s}},
		{name: "Retry-After above 60 s", answers: retryAfter("120"), wantStatus: 200, wantRequests: 2, wantWaits: []time.Duration{60 * s}},
		{name: "Retry-After too large for int64", answers: retryAfter("99999999999999999999"), wantStatus: 200, wantRequests: 2, wantWaits: []time.Duration{60 * s}},
		{name: "Retry-After words", answers: retryAfter("soon"), wantStatus: 200, wantRequests: 2, wantWaits: []time.Duration{s}},
		{name: "Retry-After fraction", answers: retryAfter("1.5"), wantStatus: 200, wantRequests: 2, wantWaits: []time.Duration{s}},
		{
			name:    "Retry-After date against the Date header",
			answers: retryAfter("Mon, 02 Jan 2006 15:04:10 GMT", "Date", date), wantStatus: 200, wantRequests: 2, wantWaits: []time.Duration{5 * s},
		},
		{
			name:    "Retry-After date in the past",
			answers: retryAfter("Mon, 02 Jan 2006 15:04:00 GMT", "Date", date), wantStatus: 200, wantRequests: 2, wantWaits: []time.Duration{0},
		},
		{
			name:    "answer of exactly the limit",
			answers: []translationtest.Answer{{Status: 200, Body: bytes.Repeat([]byte("x"), limit)}}, wantStatus: 200, wantRequests: 1,
			wantBody: string(bytes.Repeat([]byte("x"), limit)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &translationtest.Client{Answers: tt.answers}
			var w waits

			got, body, err := httppost.Post(t.Context(), client, endpoint, nil, []byte("body"), w.wait)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, got)
			assert.Equal(t, tt.wantBody, string(body))
			assert.Equal(t, tt.wantWaits, []time.Duration(w))
			requests := client.Requests()
			require.Len(t, requests, tt.wantRequests)
			for _, r := range requests {
				assert.Equal(t, "body", string(r.Body))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			}
		})
	}
}

func TestPostAnswerOverTheLimit(t *testing.T) {
	t.Parallel()

	limit := 1<<20 + 16*len("body")
	client := &translationtest.Client{Answers: []translationtest.Answer{{Status: 200, Body: bytes.Repeat([]byte("x"), limit+1)}}}

	_, _, err := httppost.Post(t.Context(), client, endpoint, nil, []byte("body"), (&waits{}).wait)

	var service *translation.ServiceError
	require.ErrorAs(t, err, &service)
	assert.Equal(t, 200, service.Status)
	assert.Equal(t, "The answer from translate.test has more than 1048640 bytes.", service.Msg)
	assert.Len(t, client.Requests(), 1)
}

func TestPostSendsTheHeader(t *testing.T) {
	t.Parallel()

	client := &translationtest.Client{Answers: []translationtest.Answer{{Status: 418, Header: http.Header{"X-Reply": {"1"}}, Body: []byte("teapot")}}}
	header := http.Header{"Authorization": {"DeepL-Auth-Key key"}, "Content-Type": {"text/plain"}}

	got, body, err := httppost.Post(t.Context(), client, endpoint, header, []byte("{}"), nil)

	require.NoError(t, err)
	assert.Equal(t, 418, got)
	assert.Equal(t, "teapot", string(body))
	requests := client.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "key", requests[0].Key)
	assert.Equal(t, "application/json", requests[0].Header.Get("Content-Type"))
	assert.Equal(t, "text/plain", header.Get("Content-Type"))
}

func TestPostFails(t *testing.T) {
	t.Parallel()

	t.Run("no response", func(t *testing.T) {
		t.Parallel()

		target := translationtest.ClosedURL(t) + "/v2?key=secret-key"

		_, _, err := httppost.Post(t.Context(), translation.DefaultHTTPClient(), target, nil, []byte("{}"), httppost.Sleep)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "translation: The request to 127.0.0.1 failed: ")
		assert.NotContains(t, err.Error(), "secret-key")
		require.ErrorIs(t, err, translation.ErrService)
		var uerr *url.Error
		require.ErrorAs(t, err, &uerr)
	})

	t.Run("bad request URL", func(t *testing.T) {
		t.Parallel()

		client := &translationtest.Client{Answers: status(200)}

		_, _, err := httppost.Post(t.Context(), client, "http://user:secret@host/%zz", nil, []byte("{}"), nil)

		var service *translation.ServiceError
		require.ErrorAs(t, err, &service)
		assert.Equal(t, 0, service.Status)
		assert.Equal(t, `The request URL is not valid: invalid URL escape "%zz".`, service.Msg)
		assert.Empty(t, client.Requests())
	})

	t.Run("context ended in the wait", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		client := &translationtest.Client{Answers: status(429)}
		wait := func(ctx context.Context, _ time.Duration) error {
			cancel()
			return ctx.Err()
		}

		_, _, err := httppost.Post(ctx, client, endpoint, nil, []byte("{}"), wait)

		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, translation.ErrService)
		assert.Len(t, client.Requests(), 1)
	})

	t.Run("sleep with an ended context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.ErrorIs(t, httppost.Sleep(ctx, time.Hour), context.Canceled)
	})
}

func TestHelpers(t *testing.T) {
	t.Parallel()

	got, err := httppost.JSON(map[string]any{"q": []string{"<x1>a</x1> & b/c", "\xff"}})
	require.NoError(t, err)
	want := "{\"q\":[\"<x1>a</x1> & b/c\",\"\ufffd\"]}"
	assert.Equal(t, want, string(got))

	for in, want := range map[string]bool{
		"http://host": true, "HTTPS://h/path": true, "http://": false, "http:///x": false, "ftp://host": false, "host": false, "": false,
	} {
		assert.Equal(t, want, httppost.ValidBaseURL(in), in)
	}
	for in, want := range map[string]bool{
		"abc": true, "a b": true, "": true, " abc": false, "abc ": false, "a\nb": false, "a\x7fb": false, "a\x00b": false, "a\tb": false,
	} {
		assert.Equal(t, want, httppost.CleanKey(in), in)
	}
	assert.Equal(t, "key *** here", httppost.Redact("key secret here", "secret"))
	assert.Equal(t, "no key", httppost.Redact("no key", ""))
}

func BenchmarkPostAnswer(b *testing.B) {
	for _, size := range []int{1 << 10, 1 << 20} {
		client := &translationtest.Client{Answers: []translationtest.Answer{{Status: 200, Body: bytes.Repeat([]byte("x"), size)}}}
		b.Run("bytes="+strconv.Itoa(size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _, err := httppost.Post(b.Context(), client, endpoint, nil, []byte("{}"), nil)
				require.NoError(b, err)
			}
		})
	}
}
