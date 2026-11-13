package openai

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/translationtest"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation"
)

const key = "sk-proj-4f1c9e2a7b3d"

func recordWaits(e *Engine) *[]time.Duration {
	var waits []time.Duration
	e.wait = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}
	return &waits
}

func TestTranslateRetriesAfterHTTP429(t *testing.T) {
	t.Parallel()

	client := &translationtest.Client{Answers: []translationtest.Answer{
		{Status: 429, Body: []byte(`{}`)},
		{Status: 200, Body: []byte(`{"choices":[{"message":{"content":"[\"Hallo\"]"}}]}`)},
	}}
	e, err := New(Options{BaseURL: "http://localhost:11434/v1", Model: "llama3", HTTPClient: client})
	require.NoError(t, err)
	waits := recordWaits(e)

	got, err := e.Translate(t.Context(), []string{"Hello"}, "en", "de")

	require.NoError(t, err)
	assert.Equal(t, []string{"Hallo"}, got)
	assert.Equal(t, []time.Duration{time.Second}, *waits)
	assert.Len(t, client.Requests(), 2)
}

func TestTranslateStatusErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		body   []byte
		want   string
	}{
		{
			"401 with the key in the message", 401, []byte(`{"error": {"message": "Incorrect API key provided: ` + key + `."}}`),
			"The OpenAI-compatible service rejected the API key (HTTP 401). Incorrect API key provided: ***.",
		},
		{
			"404 recorded", 404, testfiles.Read(t, "translation/openai_error.json"),
			"The OpenAI-compatible service answered with HTTP 404. The model `gpt-9` does not exist or you do not have access to it.",
		},
		{
			"404 of Ollama", 404, []byte(`{"error": "model \"llama3\" not found, try pulling it first"}`),
			`The OpenAI-compatible service answered with HTTP 404. model "llama3" not found, try pulling it first`,
		},
		{"a key in an error string", 400, []byte(`{"error": "bad ` + key + `"}`), "The OpenAI-compatible service answered with HTTP 400. bad ***"},
		{
			"429 after the retries", 429, []byte(`{"error": {"message": "Rate limit reached"}}`),
			"The OpenAI-compatible service got too many requests (HTTP 429). Wait and try again.",
		},
		{"500 without JSON", 500, []byte("Internal Server Error"), "The OpenAI-compatible service answered with HTTP 500."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &translationtest.Client{Answers: []translationtest.Answer{{Status: tt.status, Body: tt.body}}}
			e, err := New(Options{BaseURL: "http://localhost:11434/v1", Model: "llama3", APIKey: key, HTTPClient: client})
			require.NoError(t, err)
			recordWaits(e)

			_, err = e.Translate(t.Context(), []string{"Hello"}, "en", "de")

			var se *translation.ServiceError
			require.ErrorAs(t, err, &se)
			require.ErrorIs(t, err, translation.ErrService)
			assert.Equal(t, tt.status, se.Status)
			assert.Equal(t, tt.want, se.Msg)
			assert.NotContains(t, err.Error(), key)
		})
	}
}
