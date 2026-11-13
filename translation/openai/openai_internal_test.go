package openai

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/translationtest"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation"
)

func TestTranslateRetriesAfterHTTP429(t *testing.T) {
	t.Parallel()

	client := &translationtest.Client{Answers: []translationtest.Answer{
		{Status: 429, Body: []byte(`{}`)},
		{Status: 200, Body: chat(`[\"Hallo\"]`)},
	}}
	e, err := New(Options{BaseURL: "http://localhost:11434/v1", Model: "llama3", HTTPClient: client})
	require.NoError(t, err)
	var waits []time.Duration
	e.wait = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}

	got, err := e.Translate(t.Context(), []string{"Hello"}, "en", "de")

	require.NoError(t, err)
	assert.Equal(t, []string{"Hallo"}, got)
	assert.Equal(t, []time.Duration{time.Second}, waits)
	assert.Len(t, client.Requests(), 2)
}

func chat(content string) []byte {
	return []byte(`{"choices":[{"message":{"content":"` + content + `"}}]}`)
}

func TestTranslateStatusErrorsAfterRetries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		body   string
		want   string
	}{
		{429, `{"error": {"message": "Rate limit reached"}}`, "The OpenAI-compatible service got too many requests (HTTP 429). Wait and try again."},
		{500, "Internal Server Error", "The OpenAI-compatible service answered with HTTP 500."},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.status), func(t *testing.T) {
			t.Parallel()

			client := &translationtest.Client{Answers: []translationtest.Answer{{Status: tt.status, Body: []byte(tt.body)}}}
			e, err := New(Options{BaseURL: "http://localhost:11434/v1", Model: "llama3", HTTPClient: client})
			require.NoError(t, err)
			var waits []time.Duration
			e.wait = func(_ context.Context, d time.Duration) error {
				waits = append(waits, d)
				return nil
			}

			_, err = e.Translate(t.Context(), []string{"Hello"}, "en", "de")

			var se *translation.ServiceError
			require.ErrorAs(t, err, &se)
			assert.Equal(t, tt.status, se.Status)
			assert.Equal(t, tt.want, se.Msg)
			assert.Len(t, client.Requests(), 4)
			assert.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}, waits)
		})
	}
}
